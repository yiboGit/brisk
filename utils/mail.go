package utils

import (
	"fmt"
	"net/smtp"
)

func SendMail(to []string, subject, body string) error {
	// Set up authentication information.
	auth := smtp.PlainAuth("", "tech@epeijing.cn", "Eglass2018", "smtp.exmail.qq.com")

	// Connect to the server, authenticate, set the sender and recipient,
	// and send the email all in one step.
	msg := []byte("To: hb@epeijing.cn\r\n" +
		fmt.Sprintf("Subject: %s\r\n", subject) +
		"\r\n" +
		fmt.Sprintf("%s\r\n", body))
	return smtp.SendMail("smtp.exmail.qq.com:25", auth, "tech@epeijing.cn", to, msg)
}
