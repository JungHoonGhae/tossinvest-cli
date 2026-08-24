package output

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
	"github.com/JungHoonGhae/tossinvest-cli/internal/i18n"
)

// WriteCommunityBoards renders the community lounges ranked by followers.
// The server's order is preserved — it is the ranking.
func WriteCommunityBoards(w io.Writer, format Format, b domain.CommunityBoards) error {
	switch format {
	case FormatJSON:
		return writeJSON(w, b)
	case FormatCSV:
		var csvRows [][]string
		for i, board := range b.Boards {
			csvRows = append(csvRows, []string{
				strconv.Itoa(i + 1),
				board.Title,
				board.SubjectID,
				strconv.Itoa(board.FollowerCount),
				strconv.Itoa(board.CommentCount),
				strconv.FormatBool(board.IsMember),
				board.CreatedAt,
			})
		}
		return writeCSV(w, []string{"rank", "title", "subject_id", "followers", "comments", "is_member", "created_at"}, csvRows)
	case FormatTable:
		if _, err := fmt.Fprint(w, i18n.T("output.communityBoards.header")); err != nil {
			return err
		}
		for i, board := range b.Boards {
			mark := ""
			if board.IsMember {
				mark = i18n.T("output.communityBoards.memberMark")
			}
			line := fmt.Sprintf(i18n.T("output.communityBoards.row"),
				i+1, board.Title, board.FollowerCount, board.CommentCount, mark)
			if _, err := fmt.Fprintln(w, strings.TrimRight(line, " \n")); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}
