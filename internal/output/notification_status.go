package output

import (
	"fmt"
	"io"
	"strconv"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

// WriteNotificationStatus renders canonical preferences plus inbox and AI-agreement state.
func WriteNotificationStatus(w io.Writer, format Format, status domain.NotificationStatus) error {
	switch format {
	case FormatJSON:
		return writeJSON(w, status)
	case FormatCSV:
		return writeCSV(w,
			[]string{"inbox_unread", "ai_issue_sns_release_alert_enabled", "fomc_live_alert_enabled", "reasoning_contents_alert_enabled", "reasoning_agreement", "reasoning_news_count"},
			[][]string{{
				strconv.FormatBool(status.InboxUnread),
				strconv.FormatBool(status.AIIssueSNSReleaseAlertEnabled),
				strconv.FormatBool(status.FOMCLiveAlertEnabled),
				strconv.FormatBool(status.ReasoningContentsAlertEnabled),
				strconv.FormatBool(status.ReasoningAgreement),
				strconv.Itoa(status.ReasoningNewsCount),
			}},
		)
	case FormatTable:
		_, err := fmt.Fprintf(w,
			"Inbox unread:                    %t\nAI issue SNS release alert:       %t\nFOMC live alert:                  %t\nReasoning contents alert:         %t\nReasoning agreement:              %t\n",
			status.InboxUnread,
			status.AIIssueSNSReleaseAlertEnabled,
			status.FOMCLiveAlertEnabled,
			status.ReasoningContentsAlertEnabled,
			status.ReasoningAgreement,
		)
		return err
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}
