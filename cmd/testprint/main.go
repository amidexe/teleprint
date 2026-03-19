// cmd/testprint/main.go — утилита для тестовой печати напрямую по IP
//
// Использование:
//   go run ./cmd/testprint -host 192.168.3.57
//   go run ./cmd/testprint -host 192.168.3.57 -print file.pdf

package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"

	ipp "github.com/phin1x/go-ipp"
)

func main() {
	var (
		printFile = flag.String("print", "", "PDF файл для печати")
		host      = flag.String("host", "", "IP принтера (обязательно)")
		port      = flag.Int("port", 631, "Порт принтера")
	)
	flag.Parse()

	if *host == "" {
		log.Fatal("Укажите IP принтера: -host <IP>")
	}

	printerURL := fmt.Sprintf("http://%s:%d/ipp/print", *host, *port)
	printerURI := fmt.Sprintf("ipp://%s:%d/ipp/print", *host, *port)

	client := ipp.NewIPPClient(*host, *port, "", "", false)
	printAttributes(client, printerURL, printerURI)

	if *printFile == "" {
		fmt.Println("\n💡 Для печати: -print <файл.pdf>")
		return
	}

	printPDF(*host, *port, *printFile)
}

func printAttributes(client *ipp.IPPClient, printerURL, printerURI string) {
	fmt.Println("\n📋 Атрибуты принтера:")

	req := ipp.NewRequest(ipp.OperationGetPrinterAttributes, 1)
	req.OperationAttributes[ipp.AttributeCharset] = "utf-8"
	req.OperationAttributes[ipp.AttributeNaturalLanguage] = "en"
	req.OperationAttributes[ipp.AttributePrinterURI] = printerURI
	req.OperationAttributes["requested-attributes"] = []string{
		"printer-name", "printer-make-and-model", "printer-state",
		"printer-state-reasons", "document-format-supported",
		"copies-supported", "media-default", "color-supported", "sides-supported",
	}

	resp, err := client.SendRequest(printerURL, req, nil)
	if err != nil {
		log.Printf("  ⚠️  Ошибка атрибутов: %v", err)
		return
	}
	for _, attrs := range resp.PrinterAttributes {
		for key, val := range attrs {
			fmt.Printf("  %-35s = %v\n", key, val)
		}
	}
}

// printPDF конвертирует PDF в URF через Ghostscript и отправляет на принтер.
func printPDF(host string, port int, pdfPath string) {
	fmt.Printf("\n🖨  Печать: %s\n", pdfPath)

	// Конвертируем PDF → URF
	urfFile, err := os.CreateTemp("", "teleprint-*.urf")
	if err != nil {
		log.Fatalf("Не удалось создать временный файл: %v", err)
	}
	urfPath := urfFile.Name()
	urfFile.Close()
	defer os.Remove(urfPath)

	fmt.Println("  Конвертация PDF → URF...")
	cmd := exec.Command("gs",
		"-dSAFER", "-dBATCH", "-dNOPAUSE", "-dNOPROMPT",
		"-sDEVICE=urfgray",
		"-r600",
		"-dDEVICEWIDTHPOINTS=595",
		"-dDEVICEHEIGHTPOINTS=842",
		fmt.Sprintf("-sOutputFile=%s", urfPath),
		pdfPath,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Fatalf("Ошибка Ghostscript: %v\n%s", err, out)
	}

	sendIPP(host, port, urfPath, "image/urf")
}

// sendIPP отправляет файл на принтер через прямой IPP/HTTP запрос.
// Не использует go-ipp client напрямую, т.к. он некорректно обрабатывает
// IPP статус 1 (successful-ok-ignored-or-substituted-attributes).
func sendIPP(host string, port int, filePath, mimeType string) {
	printerURL := fmt.Sprintf("http://%s:%d/ipp/print", host, port)
	printerURI := fmt.Sprintf("ipp://%s:%d/ipp/print", host, port)

	f, err := os.Open(filePath)
	if err != nil {
		log.Fatalf("Не удалось открыть файл: %v", err)
	}
	defer f.Close()

	fi, _ := f.Stat()

	req := ipp.NewRequest(ipp.OperationPrintJob, 1)
	req.OperationAttributes[ipp.AttributeCharset] = "utf-8"
	req.OperationAttributes[ipp.AttributeNaturalLanguage] = "en"
	req.OperationAttributes[ipp.AttributePrinterURI] = printerURI
	req.OperationAttributes[ipp.AttributeRequestingUserName] = "teleprint"
	req.OperationAttributes[ipp.AttributeDocumentFormat] = mimeType
	req.OperationAttributes["job-name"] = "teleprint-test"
	req.OperationAttributes["copies"] = 1

	headerBytes, err := req.Encode()
	if err != nil {
		log.Fatalf("Ошибка кодирования IPP: %v", err)
	}

	body := io.MultiReader(bytes.NewReader(headerBytes), f)
	httpReq, _ := http.NewRequest(http.MethodPost, printerURL, body)
	httpReq.Header.Set("Content-Type", "application/ipp")
	httpReq.ContentLength = int64(len(headerBytes)) + fi.Size()

	httpResp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		log.Fatalf("❌ HTTP ошибка: %v", err)
	}
	defer httpResp.Body.Close()

	respBody, _ := io.ReadAll(httpResp.Body)
	ippResp, err := ipp.NewResponseDecoder(bytes.NewReader(respBody)).Decode(nil)
	if err != nil {
		log.Fatalf("Ошибка декодирования ответа: %v", err)
	}

	// Статусы 0-2 — успех (0=ok, 1=ok-ignored-attrs, 2=ok-conflicting-attrs)
	if ippResp.StatusCode > 2 {
		log.Fatalf("❌ IPP ошибка статус %d (0x%04X)", ippResp.StatusCode, ippResp.StatusCode)
	}

	fmt.Printf("✅ Задание отправлено (IPP статус %d)\n", ippResp.StatusCode)
	for _, jobAttrs := range ippResp.JobAttributes {
		for key, val := range jobAttrs {
			fmt.Printf("  %s = %v\n", key, val)
		}
	}
}
