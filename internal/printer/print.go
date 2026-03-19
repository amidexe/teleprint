package printer

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"

	ipp "github.com/phin1x/go-ipp"
)

// Client отправляет задания на принтер через IPP.
type Client struct {
	host string
	port int
}

func NewClient(host string, port int) *Client {
	return &Client{host: host, port: port}
}

// PrintPDF принимает путь к PDF, конвертирует его в URF через Ghostscript
// и отправляет на принтер.
// Копии реализованы через повтор страниц в URF-файле, т.к. Brother с AirPrint
// игнорирует IPP-атрибут copies (статус 1 = ignored).
func (c *Client) PrintPDF(pdfPath string, copies int, jobName string) error {
	if copies < 1 {
		copies = 1
	}
	urfPath, err := pdfToURF(pdfPath, copies)
	if err != nil {
		return fmt.Errorf("конвертация PDF→URF: %w", err)
	}
	defer os.Remove(urfPath)

	return c.sendIPP(urfPath, "image/urf", 1, jobName)
}

// pdfToURF конвертирует PDF в Apple Raster (URF) через Ghostscript.
// 600 DPI, grayscale (принтер B&W), A4.
// copies реализуется повтором входного файла — Brother с AirPrint игнорирует
// IPP-атрибут copies, поэтому страницы должны быть физически повторены в файле.
func pdfToURF(pdfPath string, copies int) (string, error) {
	tmp, err := os.CreateTemp("", "teleprint-*.urf")
	if err != nil {
		return "", err
	}
	urfPath := tmp.Name()
	tmp.Close()

	// Повторяем pdfPath copies раз как аргументы — gs обработает их последовательно
	args := []string{
		"-dSAFER", "-dBATCH", "-dNOPAUSE", "-dNOPROMPT",
		"-sDEVICE=urfgray",
		"-r600",
		"-dDEVICEWIDTHPOINTS=595",
		"-dDEVICEHEIGHTPOINTS=842",
		fmt.Sprintf("-sOutputFile=%s", urfPath),
	}
	for i := 0; i < copies; i++ {
		args = append(args, pdfPath)
	}

	cmd := exec.Command("gs", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		os.Remove(urfPath)
		return "", fmt.Errorf("ghostscript: %w\n%s", err, out)
	}

	return urfPath, nil
}

// sendIPP отправляет файл на принтер через IPP Print-Job.
// Логирует параметры задания для диагностики.
// Использует прямой HTTP-запрос: go-ipp некорректно обрабатывает
// IPP статус 1 (successful-ok-ignored-or-substituted-attributes).
func (c *Client) sendIPP(filePath, mimeType string, copies int, jobName string) error {
	printerURL := fmt.Sprintf("http://%s:%d/ipp/print", c.host, c.port)
	printerURI := fmt.Sprintf("ipp://%s:%d/ipp/print", c.host, c.port)

	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("открытие файла: %w", err)
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return err
	}

	req := ipp.NewRequest(ipp.OperationPrintJob, 1)
	req.OperationAttributes[ipp.AttributeCharset] = "utf-8"
	req.OperationAttributes[ipp.AttributeNaturalLanguage] = "en"
	req.OperationAttributes[ipp.AttributePrinterURI] = printerURI
	req.OperationAttributes[ipp.AttributeRequestingUserName] = "teleprint"
	req.OperationAttributes[ipp.AttributeDocumentFormat] = mimeType
	req.OperationAttributes["job-name"] = jobName
	// copies — Job Template атрибут, не Operation
	req.JobAttributes["copies"] = copies
	slog.Info("IPP задание", "jobName", jobName, "copies", copies, "mime", mimeType)

	headerBytes, err := req.Encode()
	if err != nil {
		return fmt.Errorf("кодирование IPP: %w", err)
	}

	body := io.MultiReader(bytes.NewReader(headerBytes), f)
	httpReq, err := http.NewRequest(http.MethodPost, printerURL, body)
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/ipp")
	httpReq.ContentLength = int64(len(headerBytes)) + fi.Size()

	httpResp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("HTTP: %w", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != 200 {
		return fmt.Errorf("HTTP статус %d", httpResp.StatusCode)
	}

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return fmt.Errorf("чтение ответа: %w", err)
	}

	ippResp, err := ipp.NewResponseDecoder(bytes.NewReader(respBody)).Decode(nil)
	if err != nil {
		return fmt.Errorf("декодирование IPP ответа: %w", err)
	}

	// Статусы 0-2 = успех (0=ok, 1=ok-ignored-attrs, 2=ok-conflicting-attrs)
	if ippResp.StatusCode > 2 {
		return fmt.Errorf("IPP ошибка статус %d (0x%04X)", ippResp.StatusCode, ippResp.StatusCode)
	}

	slog.Info("IPP ответ принтера", "status", ippResp.StatusCode)
	for _, attrs := range ippResp.JobAttributes {
		for k, v := range attrs {
			slog.Info("job attr", "key", k, "value", fmt.Sprintf("%v", v))
		}
	}
	return nil
}
