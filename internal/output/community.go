package output

import (
	"fmt"
	"io"
	"strconv"

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
		headers := []string{"RANK", "TITLE", "FOLLOWERS", "COMMENTS", ""}
		aligns := []Align{AlignRight, AlignLeft, AlignRight, AlignRight, AlignLeft}
		var rows [][]string
		for i, board := range b.Boards {
			mark := ""
			if board.IsMember {
				mark = i18n.T("output.communityBoards.memberMark")
			}
			rows = append(rows, []string{
				strconv.Itoa(i + 1),
				board.Title,
				strconv.Itoa(board.FollowerCount),
				strconv.Itoa(board.CommentCount),
				mark,
			})
		}
		return renderTable(w, headers, rows, aligns...)
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}
