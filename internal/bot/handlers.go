package bot

import (
	"fmt"
	"strconv"
	"strings"

	tele "gopkg.in/telebot.v3"
)

func (b *Bot) handleStart(c tele.Context) error {
	if b.access.IsAllowed(c.Sender().ID) {
		return c.Send(
			"🖨 *Teleprint* — бот для печати документов\n\n" +
				"Отправьте PDF, фото или изображение — и я распечатаю его на принтере.\n\n" +
				"*Поддерживаемые форматы:* PDF, JPG, PNG",
			&tele.SendOptions{ParseMode: tele.ModeMarkdown},
		)
	}
	return c.Send(
		fmt.Sprintf(
			"⛔ У вас нет доступа.\n\nВаш ID: `%d`\n\nПопросите администратора добавить вас командой `/adduser %d`",
			c.Sender().ID, c.Sender().ID,
		),
		&tele.SendOptions{ParseMode: tele.ModeMarkdown},
	)
}

func (b *Bot) handleAddUser(c tele.Context) error {
	if !b.access.IsAdmin(c.Sender().ID) {
		return c.Send("⛔ Только администратор может добавлять пользователей.")
	}

	id, err := parseUserID(c.Message().Payload)
	if err != nil {
		return c.Send("Использование: `/adduser <ID>`", &tele.SendOptions{ParseMode: tele.ModeMarkdown})
	}

	if id == b.cfg.AdminID {
		return c.Send("Администратор уже имеет доступ.")
	}

	if b.access.Add(id) {
		return c.Send(fmt.Sprintf("✅ Пользователь `%d` добавлен.", id),
			&tele.SendOptions{ParseMode: tele.ModeMarkdown})
	}
	return c.Send(fmt.Sprintf("ℹ️ Пользователь `%d` уже имеет доступ.", id),
		&tele.SendOptions{ParseMode: tele.ModeMarkdown})
}

func (b *Bot) handleDelUser(c tele.Context) error {
	if !b.access.IsAdmin(c.Sender().ID) {
		return c.Send("⛔ Только администратор может удалять пользователей.")
	}

	id, err := parseUserID(c.Message().Payload)
	if err != nil {
		return c.Send("Использование: `/deluser <ID>`", &tele.SendOptions{ParseMode: tele.ModeMarkdown})
	}

	if id == b.cfg.AdminID {
		return c.Send("❌ Нельзя удалить администратора.")
	}

	if b.access.Remove(id) {
		return c.Send(fmt.Sprintf("✅ Пользователь `%d` удалён.", id),
			&tele.SendOptions{ParseMode: tele.ModeMarkdown})
	}
	return c.Send(fmt.Sprintf("ℹ️ Пользователь `%d` не найден.", id),
		&tele.SendOptions{ParseMode: tele.ModeMarkdown})
}

func (b *Bot) handleListUsers(c tele.Context) error {
	if !b.access.IsAdmin(c.Sender().ID) {
		return c.Send("⛔ Только администратор.")
	}

	ids := b.access.List()
	if len(ids) == 0 {
		return c.Send("Список пользователей пуст. Добавьте командой `/adduser <ID>`",
			&tele.SendOptions{ParseMode: tele.ModeMarkdown})
	}

	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = fmt.Sprintf("• `%d`", id)
	}
	return c.Send(
		"*Авторизованные пользователи:*\n"+strings.Join(parts, "\n"),
		&tele.SendOptions{ParseMode: tele.ModeMarkdown},
	)
}

func parseUserID(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("пустой ID")
	}
	return strconv.ParseInt(s, 10, 64)
}
