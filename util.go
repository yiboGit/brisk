package brisk

import (
	"crypto/tls"
	"fmt"
	"net/smtp"
)

// SendMail 25端口（阿里云服务器 禁用25端口） 邮件发送 to: 收件人, subject: 邮件主题, body: 邮件具体信息
func SendMail(to []string, subject, body string) error {
	// Set up authentication information.
	auth := smtp.PlainAuth("", "tech@epeijing.cn", "Eglass2018", "smtp.exmail.qq.com")
	// Connect to the server, authenticate, set the sender and recipient,
	// and send the email all in one step.
	var mails string
	for _, mailOne := range to {
		mailStr := "To: " + mailOne + "\r\n"
		mails = mails + mailStr
	}
	msg := []byte(mails +
		fmt.Sprintf("Subject: %s\r\n", subject) +
		"\r\n" +
		fmt.Sprintf("%s\r\n", body))
	//smtpdm.aliyun.com:25
	return smtp.SendMail("smtp.exmail.qq.com:25", auth, "tech@epeijing.cn", to, msg)
}

// SendMailTLS 465端口 发送邮件 邮件发送 to: 收件人, subject: 邮件主题, body: 邮件具体信息
func SendMailTLS(to []string, subject, body string) error {
	auth := smtp.PlainAuth("", "tech@epeijing.cn", "Eglass2018", "smtp.exmail.qq.com")
	var mails string
	for _, mailOne := range to {
		mailStr := "To: " + mailOne + "\r\n"
		mails = mails + mailStr
	}
	msg := []byte(mails +
		fmt.Sprintf("Subject: %s\r\n", subject) +
		"\r\n" +
		fmt.Sprintf("%s\r\n", body))

	conn, err := tls.Dial("tcp", "smtp.exmail.qq.com:465", nil)
	if err != nil {
		return err
	}
	c, err := smtp.NewClient(conn, "smtp.exmail.qq.com")
	if err != nil {
		return err
	}
	err = c.Auth(auth)
	if err != nil {
		return err
	}
	err = c.Mail("tech@epeijing.cn")
	if err != nil {
		return err
	}
	for _, add := range to {
		if err := c.Rcpt(add); err != nil {
			return err
		}
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	_, err = w.Write(msg)
	if err != nil {
		return err
	}
	err = w.Close()
	if err != nil {
		return err
	}
	err = c.Quit()
	if err != nil {
		return err
	}
	return nil
}
