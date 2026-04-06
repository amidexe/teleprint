package converter

import (
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// OfficeToPDF конвертирует офисный документ в PDF через Gotenberg API.
// Возвращает путь к временному PDF-файлу.
func OfficeToPDF(gotenbergURL, srcPath string) (string, error) {
	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)

	// Пишем multipart в pipe в горутине
	go func() {
		defer pw.Close()
		defer writer.Close()

		part, err := writer.CreateFormFile("files", filepath.Base(srcPath))
		if err != nil {
			pw.CloseWithError(err)
			return
		}
		f, err := os.Open(srcPath)
		if err != nil {
			pw.CloseWithError(err)
			return
		}
		defer f.Close()
		if _, err := io.Copy(part, f); err != nil {
			pw.CloseWithError(err)
			return
		}
	}()

	url := gotenbergURL + "/forms/libreoffice/convert"
	req, err := http.NewRequest(http.MethodPost, url, pr)
	if err != nil {
		return "", fmt.Errorf("создание запроса: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("запрос к Gotenberg: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("Gotenberg вернул %d: %s", resp.StatusCode, body)
	}

	// Сохраняем PDF во временный файл
	tmp, err := os.CreateTemp("", "teleprint-office-*.pdf")
	if err != nil {
		return "", fmt.Errorf("tmp create: %w", err)
	}
	defer tmp.Close()

	if _, err := io.Copy(tmp, resp.Body); err != nil {
		os.Remove(tmp.Name())
		return "", fmt.Errorf("сохранение PDF: %w", err)
	}

	return tmp.Name(), nil
}
