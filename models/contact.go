package models

import (
	"fmt"
)

// Contact defines as contact as saved in the postgres table for sgs.com
type Contact struct {
	ID           string  `db:"id"`
	Name         string  `db:"name"`
	Email        string  `db:"email"`
	Phone        string  `db:"phone"`
	Message      string  `db:"message"`
	CaptchaScore float64 `db:"captcha_score"`
	Acknowledged bool    `db:"acknowledged"`
	CreatedOn    int64   `db:"created_on"`
	UpdatedOn    int64   `db:"updated_on"`
}

func (c Contact) String() string {
	return fmt.Sprintf("Contact name: %s, email: %s, phone: %s", c.Name, c.Email, c.Phone)
}
