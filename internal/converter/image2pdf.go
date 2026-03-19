package converter

import (
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"

	"github.com/disintegration/imaging"
	"github.com/go-pdf/fpdf"
)

// A4 размеры в миллиметрах
const (
	a4W = 210.0
	a4H = 297.0
)

// ImageToPDF конвертирует JPG/PNG в PDF формата A4.
// Параметры:
//   - srcPath: путь к исходному изображению
//   - rotation: поворот в градусах (0, 90, 180, 270)
//   - scalePct: масштаб в % от листа (25, 50, 75, 100)
//
// Возвращает путь к временному PDF-файлу.
func ImageToPDF(srcPath string, rotation int, scalePct int) (string, error) {
	// Открываем изображение
	src, err := imaging.Open(srcPath, imaging.AutoOrientation(true))
	if err != nil {
		return "", fmt.Errorf("открытие изображения: %w", err)
	}

	// Применяем поворот
	switch rotation {
	case 90:
		src = imaging.Rotate90(src)
	case 180:
		src = imaging.Rotate180(src)
	case 270:
		src = imaging.Rotate270(src)
	}

	bounds := src.Bounds()
	imgW := float64(bounds.Dx())
	imgH := float64(bounds.Dy())

	// Выбираем ориентацию листа под изображение
	orientation := "P" // Portrait
	pageW, pageH := a4W, a4H
	if imgW > imgH {
		orientation = "L" // Landscape
		pageW, pageH = a4H, a4W
	}

	// Вычисляем размер и позицию изображения на листе
	if scalePct <= 0 || scalePct > 100 {
		scalePct = 100
	}
	x, y, drawW, drawH := scaleOnPage(imgW, imgH, pageW, pageH, scalePct)

	// Создаём PDF
	pdf := fpdf.NewCustom(&fpdf.InitType{
		OrientationStr: orientation,
		UnitStr:        "mm",
		SizeStr:        "A4",
		FontDirStr:     "",
	})
	pdf.AddPage()
	pdf.SetMargins(0, 0, 0)
	pdf.SetAutoPageBreak(false, 0)

	// Сохраняем повёрнутое изображение во временный JPEG для fpdf
	tmpImg, err := os.CreateTemp("", "teleprint-img-*.jpg")
	if err != nil {
		return "", fmt.Errorf("tmp img: %w", err)
	}
	tmpImgPath := tmpImg.Name()
	tmpImg.Close()
	defer os.Remove(tmpImgPath)

	if err := imaging.Save(src, tmpImgPath); err != nil {
		return "", fmt.Errorf("сохранение изображения: %w", err)
	}

	// Регистрируем и рисуем изображение
	pdf.ImageOptions(tmpImgPath, x, y, drawW, drawH, false,
		fpdf.ImageOptions{ImageType: "JPEG", ReadDpi: false}, 0, "")

	// Сохраняем PDF
	tmpPDF, err := os.CreateTemp("", "teleprint-*.pdf")
	if err != nil {
		return "", fmt.Errorf("tmp pdf: %w", err)
	}
	pdfPath := tmpPDF.Name()
	tmpPDF.Close()

	if err := pdf.OutputFileAndClose(pdfPath); err != nil {
		os.Remove(pdfPath)
		return "", fmt.Errorf("запись PDF: %w", err)
	}

	return pdfPath, nil
}

// scaleOnPage вычисляет позицию и размер изображения на листе.
// scalePct=100 — вписать на весь лист с сохранением пропорций.
// scalePct=50  — вписать в 50% от размера листа, центрировать.
func scaleOnPage(imgW, imgH, pageW, pageH float64, scalePct int) (x, y, drawW, drawH float64) {
	// Доступная область = scalePct% от листа
	availW := pageW * float64(scalePct) / 100.0
	availH := pageH * float64(scalePct) / 100.0

	imgRatio := imgW / imgH
	availRatio := availW / availH

	// Вписываем изображение в доступную область с сохранением пропорций
	if imgRatio > availRatio {
		drawW = availW
		drawH = availW / imgRatio
	} else {
		drawH = availH
		drawW = availH * imgRatio
	}

	// Центрируем на листе
	x = (pageW - drawW) / 2
	y = (pageH - drawH) / 2
	return
}

// GetDimensions возвращает размеры изображения без полной загрузки в RAM.
func GetDimensions(path string) (int, int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()
	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		return 0, 0, err
	}
	return cfg.Width, cfg.Height, nil
}
