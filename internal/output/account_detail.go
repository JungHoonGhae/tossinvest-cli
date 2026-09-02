package output

import (
	"fmt"
	"io"
	"strings"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
	"github.com/JungHoonGhae/tossinvest-cli/internal/privacy"
)

// WriteAccountDetail renders the read-only 계좌관리 view. full reveals the
// account number; by default it is masked, because this output routinely gets
// pasted into issues and chat.
//
// CSV is not offered: the payload is three unrelated sections, not rows.
func WriteAccountDetail(w io.Writer, format Format, d domain.AccountDetail, full bool) error {
	number := d.Number
	name := d.Name
	if !full {
		number = privacy.AccountNumber(number)
		// accountName is the holder's real name — more sensitive than the number.
		name = privacy.Name(name)
	}

	if format == FormatJSON {
		// JSON is for machines, but the same masking applies — opting into the
		// real number should be one explicit flag, not one format switch.
		out := d
		out.Number = number
		out.Name = name
		return writeJSON(w, out)
	}

	head := number
	var meta []string
	if name != "" {
		meta = append(meta, name)
	}
	// d.Status is an opaque server code ("00"); it stays in JSON rather than
	// being printed as if it were a human-readable state.
	if len(meta) > 0 {
		head += "  (" + strings.Join(meta, " · ") + ")"
	}
	if _, err := fmt.Fprintf(w, "계좌 %s\n", head); err != nil {
		return err
	}
	if d.OpenedAt != "" {
		if _, err := fmt.Fprintf(w, "  개설일        %s\n", d.OpenedAt); err != nil {
			return err
		}
	}
	if d.LastTradedAt != "" {
		if _, err := fmt.Fprintf(w, "  최종거래일    %s\n", d.LastTradedAt); err != nil {
			return err
		}
	}

	if d.Withdrawable != nil || d.WithdrawalLimits != nil {
		if _, err := fmt.Fprint(w, "\n출금\n"); err != nil {
			return err
		}
		if a := d.Withdrawable; a != nil {
			if _, err := fmt.Fprintf(w, "  가능액        D+0 %.0f   D+1 %.0f   D+2 %.0f\n",
				a.Day0, a.Day1, a.Day2); err != nil {
				return err
			}
		}
		if l := d.WithdrawalLimits; l != nil {
			if _, err := fmt.Fprintf(w, "  한도          1회 %.0f   1일 %.0f   (오늘 사용 %.0f)\n",
				l.PerTransaction, l.PerDay, l.UsedToday); err != nil {
				return err
			}
		}
		if d.FullWithdrawalOn != "" {
			if _, err := fmt.Fprintf(w, "  전액출금 가능 %s\n", d.FullWithdrawalOn); err != nil {
				return err
			}
		}
		if d.TransferRestricted != nil && *d.TransferRestricted {
			if _, err := fmt.Fprint(w, "  ⚠ 거래목적 미확인으로 송금 한도가 제한된 상태입니다\n"); err != nil {
				return err
			}
		}
		// 심사 상태는 제한 여부와 별개로 보여준다. 제한이 안 걸렸어도 심사가
		// 반려됐거나 진행 중일 수 있고, 그게 곧 제한으로 이어진다.
		if tp := d.TradePurpose; tp != nil && (tp.Status != "" || tp.Purpose != "") {
			// 서버 코드를 그대로 낸다 — 토스가 웹에 매핑을 싣지 않아 번역하면 추측이다.
			line := fmt.Sprintf("  %-12s %s", "거래목적 심사", tp.Status)
			if tp.Purpose != "" {
				line += " (" + tp.Purpose + ")"
			}
			if _, err := fmt.Fprintln(w, line); err != nil {
				return err
			}
			if tp.RejectReason != "" {
				if _, err := fmt.Fprintf(w, "  %-12s %s (%s)\n", "반려 사유", tp.RejectReason, tp.RejectReasonType); err != nil {
					return err
				}
			}
		}
	}

	if d.MarginKR != nil || d.MarginUS != nil || d.DifferentialMargin != nil {
		if _, err := fmt.Fprint(w, "\n미수거래\n"); err != nil {
			return err
		}
		row := func(label string, m *domain.MarginStatus) error {
			if m == nil {
				return nil
			}
			state := "불가"
			if m.Receivable {
				state = "가능"
			}
			line := fmt.Sprintf("  %-12s %s", label, state)
			if m.Message != "" {
				line += "  — " + m.Message
			}
			_, err := fmt.Fprintln(w, line)
			return err
		}
		if err := row("국내", d.MarginKR); err != nil {
			return err
		}
		if err := row("해외", d.MarginUS); err != nil {
			return err
		}
		if d.DifferentialMargin != nil {
			state := "미적용"
			if *d.DifferentialMargin {
				state = "적용"
			}
			if _, err := fmt.Fprintf(w, "  %-12s %s\n", "차등증거금", state); err != nil {
				return err
			}
		}
	}

	if d.USDividendOption != nil {
		if _, err := fmt.Fprint(w, "\n미국 배당\n"); err != nil {
			return err
		}
		// The server enum is shown alongside the Korean gloss rather than
		// replaced by it: this setting decides whether a dividend is a taxable
		// cash event or more shares, and a reader checking it against the Toss
		// app should see the same token the app sends.
		gloss := map[string]string{"CASH": "현금 수령", "STOCK": "주식 재투자"}[d.USDividendOption.GiveType]
		line := fmt.Sprintf("  %-12s %s", "수령 방식", d.USDividendOption.GiveType)
		if gloss != "" {
			line += "  — " + gloss
		}
		if _, err := fmt.Fprintln(w, line); err != nil {
			return err
		}
		if d.USDividendOption.UpdatedAt != "" {
			if _, err := fmt.Fprintf(w, "  %-12s %s\n", "변경일", d.USDividendOption.UpdatedAt); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprint(w, "  (변경은 토스 앱에서만 됩니다 — 웹·CLI 에 변경 경로가 없습니다)\n"); err != nil {
			return err
		}
	}

	for _, warn := range d.Warnings {
		if _, err := fmt.Fprintf(w, "\n⚠ %s\n", warn); err != nil {
			return err
		}
	}
	if !full {
		_, err := fmt.Fprint(w, "\n(계좌번호 전체를 보려면 --full)\n")
		return err
	}
	return nil
}
