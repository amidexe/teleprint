package bot

import (
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/amidexe/teleprint/internal/access"
	"github.com/amidexe/teleprint/internal/config"
	"github.com/amidexe/teleprint/internal/printer"
	"github.com/amidexe/teleprint/internal/state"
	tele "gopkg.in/telebot.v3"
	"gopkg.in/telebot.v3/middleware"
)

// Bot объединяет все зависимости.
type Bot struct {
	tele         *tele.Bot
	cfg          *config.Config
	access       *access.Manager
	cache        *state.Cache
	printer      *printer.Client
	gotenbergURL string // пусто = конвертация office отключена
}

func New(cfg *config.Config) (*Bot, error) {
	pref := tele.Settings{
		Token:  cfg.TelegramToken,
		Poller: &tele.LongPoller{Timeout: 10 * time.Second},
	}

	// Опциональный прокси для Telegram API
	if cfg.ProxyURL != "" {
		proxyURL, err := url.Parse(cfg.ProxyURL)
		if err != nil {
			return nil, err
		}
		pref.Client = &http.Client{
			Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)},
		}
		slog.Info("Прокси активен", "url", cfg.ProxyURL)
	}

	bot, err := tele.NewBot(pref)
	if err != nil {
		return nil, err
	}

	b := &Bot{
		tele:         bot,
		cfg:          cfg,
		access:       access.NewManager(cfg.AdminID, cfg.DataDir),
		cache:        state.NewCache(cfg.JobTTLMinutes),
		printer:      printer.NewClient(cfg.PrinterHost, cfg.PrinterPort, cfg.PrinterFormat),
		gotenbergURL: cfg.GotenbergURL,
	}

	if b.gotenbergURL != "" {
		slog.Info("Конвертация офисных документов включена", "gotenberg", b.gotenbergURL)
	}

	b.registerHandlers()
	return b, nil
}

func (b *Bot) Start() {
	slog.Info("Бот запущен", "username", b.tele.Me.Username)
	b.tele.Start()
}

func (b *Bot) Stop() {
	b.tele.Stop()
}

func (b *Bot) registerHandlers() {
	// Глобальный recover от паник
	b.tele.Use(middleware.Recover())

	// /start и /help — без авторизации (чтобы отказ был понятен)
	b.tele.Handle("/start", b.handleStart)
	b.tele.Handle("/help", b.handleStart)

	// Все остальные хендлеры — только для авторизованных
	auth := b.tele.Group()
	auth.Use(b.authMiddleware)

	// Команды управления пользователями (только админ)
	auth.Handle("/adduser", b.handleAddUser)
	auth.Handle("/deluser", b.handleDelUser)
	auth.Handle("/users", b.handleListUsers)

	// Файлы и фото
	auth.Handle(tele.OnDocument, b.handleDocument)
	auth.Handle(tele.OnPhoto, b.handlePhoto)

	// Inline-кнопки (telebot v3: \f+unique -> c.Callback().Data = payload)
	auth.Handle("\f"+btnCopiesInc, b.handleCopiesInc)
	auth.Handle("\f"+btnCopiesDec, b.handleCopiesDec)
	auth.Handle("\f"+btnScale25, b.handleScale(25))
	auth.Handle("\f"+btnScale50, b.handleScale(50))
	auth.Handle("\f"+btnScale75, b.handleScale(75))
	auth.Handle("\f"+btnScale100, b.handleScale(100))
	auth.Handle("\f"+btnRotate, b.handleRotate)
	auth.Handle("\f"+btnPrint, b.handlePrint)
	auth.Handle("\f"+btnCancel, b.handleCancel)
	auth.Handle("\f"+btnNoop, b.handleNoop)
}
