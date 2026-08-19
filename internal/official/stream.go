package official

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/coder/websocket"
)

// 실시간 웹소켓 (AsyncAPI 3.0, 2026-08-19 출시). 스펙은
// docs/migration/asyncapi.latest.json, 분석은
// docs/reverse-engineering/change-analysis/2026-08-19.md.
const (
	defaultStreamURL = "wss://openapi-ws.tossinvest.com/ws/v1"

	// 서버는 **클라이언트로부터의 수신**이 180초 없으면 끊는다. 서버가 보내주는
	// 데이터는 이 타이머를 리셋하지 않으므로, 체결이 쏟아지는 중에도 PING 을 보내야
	// 한다. 스펙 권장 간격이 60초다.
	streamPingInterval = 60 * time.Second

	// 스펙이 지정한 재연결 백오프 (1s → 2s → 4s …, REST 429 대응과 동일).
	streamInitialBackoff = 1 * time.Second
	streamMaxBackoff     = 60 * time.Second
)

// ErrStreamShutdown 은 서버 배포 직전에 오는 server-shutdown 프레임이다. 장애가
// 아니라 예고된 핸드오프라 백오프를 키우지 않고 곧바로 재연결한다.
var ErrStreamShutdown = errors.New("official: server requested reconnect")

// Subscription 은 구독 선언 배열의 항목 하나다. Type 은 "trade:us" · "orderbook:kr" ·
// "personal:order", Codes 는 심볼 목록(주문 채널은 accountSeq 를 문자열로).
type Subscription struct {
	Type  string   `json:"type"`
	Codes []string `json:"codes"`
}

// StreamFrame 은 수신 프레임이다. 한 연결에 ack·데이터·에러·pong 이 섞여 오고
// top-level Type 으로만 갈린다.
type StreamFrame struct {
	Type       string          `json:"type"` // subscriptions | message | error | pong
	Topic      string          `json:"topic,omitempty"`
	Data       json.RawMessage `json:"data,omitempty"`
	Subscribed []string        `json:"subscribed,omitempty"`
	Rejected   []StreamReject  `json:"rejected,omitempty"`
	Error      *StreamError    `json:"error,omitempty"`
}

type StreamReject struct {
	Target  string `json:"target"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type StreamError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// WithStreamURL overrides the websocket endpoint (used in tests).
func WithStreamURL(u string) Option {
	return func(c *Client) { c.streamURL = u }
}

// Stream opens one websocket connection, declares subs, and calls handler for
// every frame until the context is cancelled or the connection drops.
//
// 구독은 **선언형 full-replace** 다 — 배열 하나가 곧 현재 구독 전체이고,
// subscribe/unsubscribe 액션은 없다. 그래서 여러 채널을 한 연결에 담는다.
func (c *Client) Stream(ctx context.Context, subs []Subscription, handler func(StreamFrame)) error {
	if len(subs) == 0 {
		return errors.New("official: no subscriptions declared")
	}
	tok, err := c.tm.token(ctx)
	if err != nil {
		return err
	}

	conn, resp, err := websocket.Dial(ctx, c.streamURLOrDefault(), &websocket.DialOptions{
		HTTPClient: c.hc,
		HTTPHeader: http.Header{"Authorization": {"Bearer " + tok}},
	})
	if err != nil {
		// 인증·허용 IP 실패는 handshake 의 HTTP 상태로만 온다 (101 이 아니면 본문 없음).
		if resp != nil {
			return fmt.Errorf("official: websocket handshake HTTP %d: %w", resp.StatusCode, err)
		}
		return fmt.Errorf("official: websocket dial: %w", err)
	}
	// 시세는 프레임이 작지만 주문 스냅샷은 크다. 기본 32KiB 로는 잘린다.
	conn.SetReadLimit(1 << 20)
	defer conn.CloseNow()

	decl, err := json.Marshal(subs)
	if err != nil {
		return fmt.Errorf("official: encode subscriptions: %w", err)
	}
	if err := conn.Write(ctx, websocket.MessageText, decl); err != nil {
		return fmt.Errorf("official: declare subscriptions: %w", err)
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go pingLoop(ctx, conn)

	for {
		typ, raw, err := conn.Read(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("official: read: %w", err)
		}
		if typ != websocket.MessageText {
			continue
		}
		var frame StreamFrame
		if err := json.Unmarshal(raw, &frame); err != nil {
			continue // 순수 텍스트 keepalive 응답 등 JSON 이 아닌 프레임
		}
		handler(frame)
		if frame.Type == "error" && frame.Error != nil && frame.Error.Code == "server-shutdown" {
			return ErrStreamShutdown
		}
	}
}

// pingLoop sends the plain-text uppercase PING keepalive. JSON 이 아니다.
func pingLoop(ctx context.Context, conn *websocket.Conn) {
	t := time.NewTicker(streamPingInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := conn.Write(ctx, websocket.MessageText, []byte("PING")); err != nil {
				return
			}
		}
	}
}

// StreamWithRetry reconnects with exponential backoff and re-declares subs.
//
// 무손실 보장은 연결 세션 내부에 한정된다 — 끊긴 구간의 주문 이벤트는 재전송되지
// 않으므로, 호출자는 재연결 후 `GET /api/v1/orders` 로 재동기화해야 한다.
func (c *Client) StreamWithRetry(ctx context.Context, subs []Subscription, handler func(StreamFrame), logf func(string, ...any)) error {
	if logf == nil {
		logf = func(string, ...any) {}
	}
	backoff := streamInitialBackoff
	for {
		err := c.Stream(ctx, subs, handler)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if errors.Is(err, ErrStreamShutdown) {
			// 예고된 배포 핸드오프 — 백오프를 키우지 않는다.
			logf("stream: server requested reconnect")
			backoff = streamInitialBackoff
		} else {
			logf("stream: disconnected, reconnecting in %s: %v", backoff, err)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}
		if backoff < streamMaxBackoff {
			backoff *= 2
		}
	}
}

func (c *Client) streamURLOrDefault() string {
	if c.streamURL != "" {
		return c.streamURL
	}
	return defaultStreamURL
}

// StreamSubscriptions builds the declaration array from per-channel symbol
// lists. 시장 구분(kr/us)은 심볼 모양으로 정한다 — KRX 는 6자리 숫자, US 는 티커.
func StreamSubscriptions(trade, orderbook []string, accountSeqs []string) []Subscription {
	var subs []Subscription
	for _, ch := range []struct {
		prefix  string
		symbols []string
	}{{"trade", trade}, {"orderbook", orderbook}} {
		byMarket := map[string][]string{}
		for _, s := range ch.symbols {
			s = strings.TrimSpace(s)
			if s == "" {
				continue
			}
			byMarket[marketOf(s)] = append(byMarket[marketOf(s)], strings.ToUpper(s))
		}
		for _, m := range []string{"kr", "us"} {
			if codes := byMarket[m]; len(codes) > 0 {
				subs = append(subs, Subscription{Type: ch.prefix + ":" + m, Codes: codes})
			}
		}
	}
	if len(accountSeqs) > 0 {
		subs = append(subs, Subscription{Type: "personal:order", Codes: accountSeqs})
	}
	return subs
}

// marketOf classifies a symbol: KRX codes are 6 digits, everything else is US.
func marketOf(symbol string) string {
	if len(symbol) != 6 {
		return "us"
	}
	for _, r := range symbol {
		if r < '0' || r > '9' {
			return "us"
		}
	}
	return "kr"
}
