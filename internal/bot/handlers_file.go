package bot

import (
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/amidexe/teleprint/internal/converter"
	"github.com/amidexe/teleprint/internal/state"
	tele "gopkg.in/telebot.v3"
)

// handleDocument принимает PDF и изображения присланные как файл.
func (b *Bot) handleDocument(c tele.Context) error {
	doc := c.Message().Document
	if doc == nil {
		return nil
	}
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(doc.FileName), "."))
	switch ext {
	case "pdf", "jpg", "jpeg", "png":
	default:
		return c.Send(fmt.Sprintf(
			"❌ Формат *.%s* не поддерживается.\n\nПоддерживаются: PDF, JPG, PNG", ext),
			&tele.SendOptions{ParseMode: tele.ModeMarkdown})
	}
	return b.processFile(c, doc.FileID, doc.FileName, ext, 0, 0)
}

// handlePhoto принимает фотографии (Telegram сжимает их в JPG).
func (b *Bot) handlePhoto(c tele.Context) error {
	photo := c.Message().Photo
	if photo == nil {
		return nil
	}
	return b.processFile(c, photo.FileID, "photo.jpg", "jpg", photo.Width, photo.Height)
}

// processFile скачивает файл, создаёт PrintJob и показывает inline-меню.
func (b *Bot) processFile(c tele.Context, fileID, fileName, fileType string, w, h int) error {
	msg, err := c.Bot().Send(c.Recipient(), "⏳ Получаю файл...")
	if err != nil {
		return err
	}

	tmpPath, err := b.downloadFile(c, fileID, fileType)
	if err != nil {
		slog.Error("Ошибка скачивания", "err", err)
		_, _ = c.Bot().Edit(msg, "❌ Не удалось скачать файл. Попробуйте ещё раз.")
		return nil
	}

	// Для изображений без размеров (Document) — читаем заголовок
	if (fileType == "jpg" || fileType == "jpeg" || fileType == "png") && (w == 0 || h == 0) {
		w, h = readImageDimensions(tmpPath)
	}

	job := &state.PrintJob{
		FileID:    fileID,
		FileName:  fileName,
		FileType:  fileType,
		ChatID:    c.Chat().ID,
		MessageID: msg.ID,
		TempPath:  tmpPath,
		Copies:    1,
		Scale:     100,
		Rotation:  0,
		ImgWidth:  w,
		ImgHeight: h,
	}
	b.cache.Set(job)

	keyboard := jobKeyboard(job)
	_, err = c.Bot().Edit(msg,
		jobCaption(job),
		&tele.SendOptions{
			ParseMode:   tele.ModeMarkdown,
			ReplyMarkup: keyboard,
		},
	)
	return err
}

// downloadFile скачивает файл из Telegram во временную директорию.
func (b *Bot) downloadFile(c tele.Context, fileID, ext string) (string, error) {
	tgFile, err := c.Bot().FileByID(fileID)
	if err != nil {
		return "", fmt.Errorf("file info: %w", err)
	}
	rc, err := c.Bot().File(&tgFile)
	if err != nil {
		return "", fmt.Errorf("file open: %w", err)
	}
	defer rc.Close()

	tmp, err := os.CreateTemp("", fmt.Sprintf("teleprint-*.%s", ext))
	if err != nil {
		return "", fmt.Errorf("tmp create: %w", err)
	}
	defer tmp.Close()

	if _, err := io.Copy(tmp, rc); err != nil {
		os.Remove(tmp.Name())
		return "", fmt.Errorf("download: %w", err)
	}
	return tmp.Name(), nil
}

// readImageDimensions читает только заголовок файла для получения размеров.
func readImageDimensions(path string) (int, int) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0
	}
	defer f.Close()
	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		return 0, 0
	}
	return cfg.Width, cfg.Height
}

// parseJobKey разбирает payload кнопки "chatID:messageID".
func parseJobKey(data string) (chatID int64, messageID int, ok bool) {
	parts := strings.SplitN(data, ":", 2)
	if len(parts) != 2 {
		return 0, 0, false
	}
	cid, err1 := strconv.ParseInt(parts[0], 10, 64)
	mid, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil {
		return 0, 0, false
	}
	return cid, mid, true
}

// editJobMenu обновляет сообщение с кнопками.
func (b *Bot) editJobMenu(c tele.Context, job *state.PrintJob) error {
	return c.Edit(
		jobCaption(job),
		&tele.SendOptions{
			ParseMode:   tele.ModeMarkdown,
			ReplyMarkup: jobKeyboard(job),
		},
	)
}

// --- Обработчики кнопок ---

func (b *Bot) handleCopiesInc(c tele.Context) error {
	_ = c.Respond()
	job, ok := b.jobFromCallback(c)
	if !ok {
		return c.Edit("⏰ Время сессии истекло. Отправьте файл заново.")
	}
	if job.Copies < b.cfg.MaxCopies {
		job.Copies++
	}
	return b.editJobMenu(c, job)
}

func (b *Bot) handleCopiesDec(c tele.Context) error {
	_ = c.Respond()
	job, ok := b.jobFromCallback(c)
	if !ok {
		return c.Edit("⏰ Время сессии истекло. Отправьте файл заново.")
	}
	if job.Copies > 1 {
		job.Copies--
	}
	return b.editJobMenu(c, job)
}

func (b *Bot) handleScale(pct int) tele.HandlerFunc {
	return func(c tele.Context) error {
		_ = c.Respond()
		job, ok := b.jobFromCallback(c)
		if !ok {
			return c.Edit("⏰ Время сессии истекло. Отправьте файл заново.")
		}
		job.Scale = pct
		return b.editJobMenu(c, job)
	}
}

func (b *Bot) handleRotate(c tele.Context) error {
	_ = c.Respond()
	job, ok := b.jobFromCallback(c)
	if !ok {
		return c.Edit("⏰ Время сессии истекло. Отправьте файл заново.")
	}
	job.Rotation = (job.Rotation + 90) % 360
	return b.editJobMenu(c, job)
}

func (b *Bot) handleCancel(c tele.Context) error {
	_ = c.Respond()
	job, ok := b.jobFromCallback(c)
	if !ok {
		return c.Edit("❌ Отменено.")
	}
	b.cache.Delete(job.ChatID, job.MessageID)
	os.Remove(job.TempPath)
	return c.Edit("❌ Отменено.")
}

func (b *Bot) handlePrint(c tele.Context) error {
	_ = c.Respond()
	job, ok := b.jobFromCallback(c)
	if !ok {
		return c.Edit("⏰ Время сессии истекло. Отправьте файл заново.")
	}
	b.cache.Delete(job.ChatID, job.MessageID)

	// Показываем статус "печатаю"
	_ = c.Edit(fmt.Sprintf(
		"🖨 Отправляю на печать...\n\nФайл: *%s*\nКопий: %d",
		escapeMarkdown(job.FileName), job.Copies,
	), &tele.SendOptions{ParseMode: tele.ModeMarkdown})

	go b.doPrint(c, job)
	return nil
}

// doPrint выполняет конвертацию и отправку задания в фоне.
func (b *Bot) doPrint(c tele.Context, job *state.PrintJob) {
	defer os.Remove(job.TempPath)

	pdfPath := job.TempPath

	// Если изображение — конвертируем в PDF
	if job.IsImage() {
		converted, err := converter.ImageToPDF(job.TempPath, job.Rotation, job.Scale)
		if err != nil {
			slog.Error("Ошибка конвертации", "err", err)
			_, _ = c.Bot().Send(c.Recipient(), "❌ Ошибка конвертации изображения.")
			return
		}
		defer os.Remove(converted)
		pdfPath = converted
	}

	// Отправляем на принтер
	err := b.printer.PrintPDF(pdfPath, job.Copies, job.FileName)
	if err != nil {
		slog.Error("Ошибка печати", "err", err, "file", job.FileName)
		_, _ = c.Bot().Send(c.Recipient(),
			fmt.Sprintf("❌ Принтер недоступен или вернул ошибку:\n`%v`", err),
			&tele.SendOptions{ParseMode: tele.ModeMarkdown})
		return
	}

	_, _ = c.Bot().Send(c.Recipient(), fmt.Sprintf(
		"✅ *%s* отправлен на печать (%d коп.)",
		escapeMarkdown(job.FileName), job.Copies,
	), &tele.SendOptions{ParseMode: tele.ModeMarkdown})
}

func (b *Bot) handleNoop(c tele.Context) error {
	return c.Respond()
}

// jobFromCallback извлекает PrintJob из payload кнопки.
func (b *Bot) jobFromCallback(c tele.Context) (*state.PrintJob, bool) {
	raw := c.Callback().Data
	slog.Debug("callback data", "raw", fmt.Sprintf("%q", raw))
	chatID, messageID, ok := parseJobKey(raw)
	if !ok {
		slog.Warn("parseJobKey failed", "raw", fmt.Sprintf("%q", raw))
		return nil, false
	}
	job, found := b.cache.Get(chatID, messageID)
	slog.Debug("cache lookup", "chatID", chatID, "messageID", messageID, "found", found)
	return job, found
}
