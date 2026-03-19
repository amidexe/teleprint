package bot

import (
	"fmt"

	"github.com/amidexe/teleprint/internal/state"
	tele "gopkg.in/telebot.v3"
)

// Уникальные идентификаторы кнопок (используются как endpoint в telebot v3).
// Payload кнопки: "chatID:messageID"
const (
	btnCopiesInc = "ci"
	btnCopiesDec = "cd"
	btnScale25   = "s25"
	btnScale50   = "s50"
	btnScale75   = "s75"
	btnScale100  = "s100"
	btnRotate    = "ro"
	btnPrint     = "pr"
	btnCancel    = "ca"
	btnNoop      = "no"
)

// jobKeyboard строит inline-клавиатуру для настройки задания печати.
func jobKeyboard(job *state.PrintJob) *tele.ReplyMarkup {
	menu := &tele.ReplyMarkup{}
	payload := fmt.Sprintf("%d:%d", job.ChatID, job.MessageID)

	// Строка копий
	rows := []tele.Row{
		menu.Row(
			menu.Data("➖", btnCopiesDec, payload),
			menu.Data(fmt.Sprintf("📄 %d коп.", job.Copies), btnNoop, payload),
			menu.Data("➕", btnCopiesInc, payload),
		),
	}

	// Масштаб и поворот — только для изображений
	if job.IsImage() {
		s := job.Scale
		label := func(pct int) string {
			if s == pct {
				return fmt.Sprintf("·%d%%·", pct)
			}
			return fmt.Sprintf("%d%%", pct)
		}
		rows = append(rows,
			menu.Row(
				menu.Data(label(25), btnScale25, payload),
				menu.Data(label(50), btnScale50, payload),
				menu.Data(label(75), btnScale75, payload),
				menu.Data(label(100), btnScale100, payload),
			),
			menu.Row(
				menu.Data(fmt.Sprintf("🔄 Повернуть (+%d°)", job.Rotation), btnRotate, payload),
			),
		)
	}

	rows = append(rows, menu.Row(
		menu.Data("❌ Отмена", btnCancel, payload),
		menu.Data("🖨 Печать", btnPrint, payload),
	))

	menu.Inline(rows...)
	return menu
}

// jobCaption формирует текст над кнопками.
func jobCaption(job *state.PrintJob) string {
	text := fmt.Sprintf("📄 *%s*\n", escapeMarkdown(job.FileName))
	if job.IsImage() && job.ImgWidth > 0 {
		text += fmt.Sprintf("Размер: %d×%d px\n", job.ImgWidth, job.ImgHeight)
		text += fmt.Sprintf("Масштаб: %d%% листа A4\n", job.Scale)
	}
	text += "\nНастройте параметры печати:"
	return text
}

func escapeMarkdown(s string) string {
	for _, ch := range []string{"_", "*", "[", "`"} {
		for i := 0; i < len(s); i++ {
			if string(s[i]) == ch {
				s = s[:i] + `\` + s[i:]
				i++
			}
		}
	}
	return s
}
