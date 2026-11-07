package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	tossclient "github.com/JungHoonGhae/tossinvest-cli/internal/client"
	"github.com/JungHoonGhae/tossinvest-cli/internal/i18n"
	"github.com/JungHoonGhae/tossinvest-cli/internal/official"
	"github.com/JungHoonGhae/tossinvest-cli/internal/tui"
	"github.com/spf13/cobra"
)

const (
	keyIssueApprovalTimeout = 5 * time.Minute
	keyIssuePollInterval    = 2 * time.Second
)

// keySetupState is what `openapi setup` found before asking anything.
type keySetupState int

const (
	keyConnected   keySetupState = iota // the saved key is Toss's key and it works
	keyConnectedIP                      // the saved key is Toss's key; only the IP allow list blocks it
	keyServerOnly                       // Toss has a key this computer cannot use
	keyMissing                          // no key at Toss yet
)

// classifyKeySetup decides the starting point. probe is nil when the saved key
// is absent or differs from Toss's key (validating it would prove nothing).
func classifyKeySetup(serverKey string, local *official.Credentials, probe *probeResult) keySetupState {
	if serverKey == "" {
		return keyMissing
	}
	if local != nil && local.APIKey == serverKey && probe != nil {
		switch {
		case probe.OK:
			return keyConnected
		case probe.ErrorKind == "ip_not_allowed":
			return keyConnectedIP
		}
	}
	return keyServerOnly
}

// keySetupClient is the WTS surface setup needs (satisfied by the hybrid client).
type keySetupClient interface {
	OpenAPIClientInfo(ctx context.Context) (tossclient.OpenAPIClientInfo, error)
	OpenAPIKeyTerms(ctx context.Context) ([]tossclient.OpenAPITerm, error)
	StartOpenAPIKeyIssue(ctx context.Context, kind tossclient.OpenAPIIssueKind) error
	OpenAPIKeyIssueStatus(ctx context.Context, kind tossclient.OpenAPIIssueKind) (tossclient.ExtensionStatus, error)
	CompleteOpenAPIKeyIssue(ctx context.Context, kind tossclient.OpenAPIIssueKind) (tossclient.IssuedOpenAPIKey, error)
}

func newOpenAPISetupCmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:         "setup",
		Short:       i18n.T("openapi.setup.short"),
		Annotations: map[string]string{"source": "wts"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runOpenAPISetup(cmd, opts)
		},
	}
}

func runOpenAPISetup(cmd *cobra.Command, opts *rootOptions) error {
	out := cmd.OutOrStdout()
	// Secrets and an irreversible reissue: only with a person at the terminal.
	if !tui.IsInteractive(os.Stdin, os.Stdout) {
		_, err := fmt.Fprint(out, i18n.T("openapi.setup.msg.nonInteractive"))
		return err
	}
	credFile, tokenFile, err := resolveOpenAPIPaths(opts)
	if err != nil {
		return err
	}
	app, err := newAppContext(opts)
	if err != nil {
		return err
	}
	ctx := cmd.Context()
	info, err := app.client.OpenAPIClientInfo(ctx)
	if err != nil {
		fmt.Fprint(out, i18n.T("openapi.setup.msg.needSession"))
		return err
	}

	local, _ := official.LoadCredentials(os.Getenv, credFile)
	var probe *probeResult
	if local != nil && local.APIKey == info.APIKey && info.APIKey != "" {
		if res, perr := validateOpenAPICredentials(ctx, *local, tokenFile); perr == nil {
			probe = &res
		}
	}

	masked := official.Credentials{APIKey: info.APIKey}.MaskedKey()
	switch classifyKeySetup(info.APIKey, local, probe) {
	case keyConnected:
		_, err := fmt.Fprintf(out, i18n.T("openapi.setup.msg.connected"), masked, formatKeyDate(info.ExpiresAt))
		return err
	case keyConnectedIP:
		_, err := fmt.Fprintf(out, i18n.T("openapi.setup.msg.connectedIP"), masked)
		return err
	case keyServerOnly:
		fmt.Fprintf(out, i18n.T("openapi.setup.msg.serverKey"), masked, info.Status, formatKeyDate(info.ExpiresAt))
		choice, err := tui.Select(i18n.T("openapi.setup.prompt.choose"), []string{
			i18n.T("openapi.setup.choice.enterSecret"),
			i18n.T("openapi.setup.choice.reissue"),
			i18n.T("openapi.setup.choice.later"),
		})
		if err != nil {
			return err
		}
		switch choice {
		case i18n.T("openapi.setup.choice.enterSecret"):
			return saveExistingKeySecret(ctx, out, info.APIKey, credFile, tokenFile)
		case i18n.T("openapi.setup.choice.reissue"):
			return issueOpenAPIKey(ctx, out, app.client, tossclient.OpenAPIReissue, masked, credFile, tokenFile)
		}
	default: // keyMissing
		fmt.Fprint(out, i18n.T("openapi.setup.msg.noKey"))
		choice, err := tui.Select(i18n.T("openapi.setup.prompt.choose"), []string{
			i18n.T("openapi.setup.choice.issue"),
			i18n.T("openapi.setup.choice.later"),
		})
		if err != nil {
			return err
		}
		if choice == i18n.T("openapi.setup.choice.issue") {
			return issueOpenAPIKey(ctx, out, app.client, tossclient.OpenAPIRegistration, "", credFile, tokenFile)
		}
	}
	_, err = fmt.Fprint(out, i18n.T("openapi.setup.msg.later"))
	return err
}

// saveExistingKeySecret pairs Toss's key with a secret the user kept. It is
// saved only when Toss accepts the pair (an IP block still proves the pair).
func saveExistingKeySecret(ctx context.Context, out io.Writer, apiKey, credFile, tokenFile string) error {
	secret, err := tui.Password(i18n.T("openapi.setup.prompt.secret"))
	if err != nil {
		return err
	}
	creds := official.Credentials{APIKey: apiKey, SecretKey: strings.TrimSpace(secret)}
	res, err := validateOpenAPICredentials(ctx, creds, tokenFile)
	if err != nil {
		return err
	}
	if !res.OK && res.ErrorKind != "ip_not_allowed" {
		if res.ErrorKind == "auth" {
			_, err := fmt.Fprint(out, i18n.T("openapi.setup.msg.secretRejected"))
			return err
		}
		_, err := fmt.Fprintf(out, i18n.T("openapi.setup.msg.verifyFailed"), res.Message)
		return err
	}
	return saveIssuedCredentials(out, creds, credFile, tokenFile, res)
}

// issueOpenAPIKey runs the web's certificate flow: show the terms, confirm
// (typed confirmation for a reissue, which revokes oldKey), push, wait for the
// phone, complete, then save and verify.
func issueOpenAPIKey(ctx context.Context, out io.Writer, c keySetupClient, kind tossclient.OpenAPIIssueKind, oldKey, credFile, tokenFile string) error {
	fmt.Fprint(out, i18n.T("openapi.setup.msg.terms"))
	if terms, err := c.OpenAPIKeyTerms(ctx); err == nil && len(terms) > 0 {
		for _, term := range terms {
			fmt.Fprintf(out, "  • %s\n    %s\n", term.Title, term.URL)
		}
	} else {
		fmt.Fprintln(out, "  • 오픈 API 서비스 이용 약관\n  • 개인(신용)정보 수집·이용 동의(오픈 API 서비스)")
	}
	fmt.Fprintln(out)

	if kind == tossclient.OpenAPIReissue {
		fmt.Fprintf(out, i18n.T("openapi.setup.msg.reissueWarning"), oldKey)
		word := i18n.T("openapi.setup.confirmWord")
		if _, err := tui.Input(i18n.T("openapi.setup.prompt.reissueConfirm"), "", func(v string) error {
			if strings.TrimSpace(v) != word {
				return errors.New(i18n.T("openapi.setup.err.reissueConfirm"))
			}
			return nil
		}); err != nil {
			return err
		}
	} else {
		agree, err := tui.Confirm(i18n.T("openapi.setup.prompt.issueConfirm"))
		if err != nil {
			return err
		}
		if !agree {
			_, err := fmt.Fprint(out, i18n.T("openapi.setup.msg.later"))
			return err
		}
	}

	if err := c.StartOpenAPIKeyIssue(ctx, kind); err != nil {
		return err
	}
	fmt.Fprint(out, i18n.T("openapi.setup.msg.pushSent"))

	// The wait reports through a channel: if the spinner is quit (Ctrl+C)
	// before the phone approved, there is no result and nothing is completed.
	done := make(chan error, 1)
	waitCtx, cancelWait := context.WithCancel(ctx)
	defer cancelWait()
	waitErr := tui.WithSpinner(i18n.T("openapi.setup.msg.waiting"), func() error {
		done <- waitForKeyIssueApproval(waitCtx, c, kind, keyIssueApprovalTimeout, keyIssuePollInterval)
		return nil
	})
	var outcome error
	select {
	case outcome = <-done:
	default:
		cancelWait()
		outcome = context.Canceled
	}
	if waitErr != nil && outcome == nil {
		return waitErr
	}
	switch {
	case errors.Is(outcome, context.Canceled):
		_, err := fmt.Fprint(out, i18n.T("openapi.setup.msg.cancelled"))
		return err
	case errors.Is(outcome, errKeyIssueRejected):
		_, err := fmt.Fprint(out, i18n.T("openapi.setup.msg.rejected"))
		return err
	case errors.Is(outcome, errKeyIssueTimeout):
		_, err := fmt.Fprint(out, i18n.T("openapi.setup.msg.timeout"))
		return err
	case outcome != nil:
		return outcome
	}

	key, err := c.CompleteOpenAPIKeyIssue(ctx, kind)
	if errors.Is(err, tossclient.ErrIssuedKeyMissing) {
		fmt.Fprint(out, i18n.T("openapi.setup.msg.issuedButMissing"))
		return err
	}
	if err != nil {
		return err
	}
	creds := official.Credentials{APIKey: key.APIKey, SecretKey: key.SecretKey}
	res, err := validateOpenAPICredentials(ctx, creds, tokenFile)
	if err != nil {
		// The pair is real either way: never lose a secret Toss won't show again.
		res = probeResult{ErrorKind: "transport_error", Message: err.Error()}
	}
	return saveIssuedCredentials(out, creds, credFile, tokenFile, res)
}

var (
	errKeyIssueRejected = errors.New("open api key issuance was declined or expired")
	errKeyIssueTimeout  = errors.New("open api key issuance was not approved in time")
)

// waitForKeyIssueApproval polls until the phone approves. Transient status
// errors are retried until the deadline.
func waitForKeyIssueApproval(ctx context.Context, c keySetupClient, kind tossclient.OpenAPIIssueKind, timeout, every time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		st, err := c.OpenAPIKeyIssueStatus(ctx, kind)
		if err == nil && st.Approved() {
			return nil
		}
		if err == nil && st.Rejected() {
			return errKeyIssueRejected
		}
		if time.Now().After(deadline) {
			return errKeyIssueTimeout
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(every):
		}
	}
}

// saveIssuedCredentials stores a pair Toss accepted (or one it just issued)
// and reports the connection check. A previous token for other credentials
// must not be reused.
func saveIssuedCredentials(out io.Writer, creds official.Credentials, credFile, tokenFile string, res probeResult) error {
	if err := saveOpenAPICredentials(credFile, creds.APIKey, creds.SecretKey); err != nil {
		return err
	}
	_ = os.Remove(tokenFile)
	fmt.Fprintf(out, i18n.T("openapi.setup.msg.saved"), creds.MaskedKey())
	switch {
	case res.OK:
		_, err := fmt.Fprint(out, i18n.T("openapi.setup.msg.verified"))
		return err
	case res.ErrorKind == "ip_not_allowed":
		_, err := fmt.Fprint(out, i18n.T("openapi.setup.msg.ipNotAllowed"))
		return err
	default:
		_, err := fmt.Fprintf(out, i18n.T("openapi.setup.msg.verifyFailed"), res.Message)
		return err
	}
}

func formatKeyDate(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.In(tossclient.KoreaLocation).Format("2006-01-02")
}
