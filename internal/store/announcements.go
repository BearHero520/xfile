package store

import (
	"database/sql"
	"errors"
	"strings"
	"unicode/utf8"

	"xfile/internal/domain"
)

func (s *Store) Announcements(publishedOnly bool) ([]domain.Announcement, error) {
	query := `SELECT id, title, content, published, created_at, updated_at FROM announcements`
	if publishedOnly {
		query += ` WHERE published = 1`
	}
	query += ` ORDER BY updated_at DESC, id DESC`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.Announcement, 0)
	for rows.Next() {
		item, err := scanAnnouncement(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) CreateAnnouncement(input domain.AnnouncementInput) (domain.Announcement, error) {
	input, err := normalizeAnnouncementInput(input)
	if err != nil {
		return domain.Announcement{}, err
	}
	published := 0
	if input.Published {
		published = 1
	}
	result, err := s.db.Exec(
		`INSERT INTO announcements(title, content, published) VALUES(?, ?, ?)`,
		input.Title,
		input.Content,
		published,
	)
	if err != nil {
		return domain.Announcement{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return domain.Announcement{}, err
	}
	return s.announcementByID(id)
}

func (s *Store) UpdateAnnouncement(id int64, input domain.AnnouncementInput) (domain.Announcement, error) {
	input, err := normalizeAnnouncementInput(input)
	if err != nil {
		return domain.Announcement{}, err
	}
	published := 0
	if input.Published {
		published = 1
	}
	result, err := s.db.Exec(
		`UPDATE announcements
		 SET title = ?, content = ?, published = ?, updated_at = STRFTIME('%Y-%m-%dT%H:%M:%fZ', 'now')
		 WHERE id = ?`,
		input.Title,
		input.Content,
		published,
		id,
	)
	if err != nil {
		return domain.Announcement{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return domain.Announcement{}, err
	}
	if affected == 0 {
		return domain.Announcement{}, sql.ErrNoRows
	}
	return s.announcementByID(id)
}

func (s *Store) DeleteAnnouncement(id int64) error {
	result, err := s.db.Exec(`DELETE FROM announcements WHERE id = ?`, id)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) announcementByID(id int64) (domain.Announcement, error) {
	return scanAnnouncement(s.db.QueryRow(
		`SELECT id, title, content, published, created_at, updated_at FROM announcements WHERE id = ?`,
		id,
	))
}

type announcementScanner interface {
	Scan(dest ...any) error
}

func scanAnnouncement(scanner announcementScanner) (domain.Announcement, error) {
	var item domain.Announcement
	var published int
	if err := scanner.Scan(
		&item.ID,
		&item.Title,
		&item.Content,
		&published,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return domain.Announcement{}, err
	}
	item.Published = published == 1
	return item, nil
}

func normalizeAnnouncementInput(input domain.AnnouncementInput) (domain.AnnouncementInput, error) {
	input.Title = strings.TrimSpace(input.Title)
	input.Content = strings.TrimSpace(input.Content)
	if input.Title == "" {
		return domain.AnnouncementInput{}, errors.New("公告标题不能为空")
	}
	if utf8.RuneCountInString(input.Title) > 120 {
		return domain.AnnouncementInput{}, errors.New("公告标题不能超过 120 个字符")
	}
	if input.Content == "" {
		return domain.AnnouncementInput{}, errors.New("公告内容不能为空")
	}
	if utf8.RuneCountInString(input.Content) > 10000 {
		return domain.AnnouncementInput{}, errors.New("公告内容不能超过 10000 个字符")
	}
	return input, nil
}
