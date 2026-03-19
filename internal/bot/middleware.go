package bot

import (
	"log/slog"

	tele "gopkg.in/telebot.v3"
)

// authMiddleware отклоняет сообщения от неавторизованных пользователей.
func (b *Bot) authMiddleware(next tele.HandlerFunc) tele.HandlerFunc {
	return func(c tele.Context) error {
		userID := c.Sender().ID
		if !b.access.IsAllowed(userID) {
			slog.Warn("Отказ в доступе", "user_id", userID, "username", c.Sender().Username)
			return c.Send("⛔ У вас нет доступа к этому боту.")
		}
		return next(c)
	}
}
