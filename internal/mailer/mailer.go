package mailer

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/elorenzorodz/event-mrs/internal/database"
	"github.com/mailgun/mailgun-go/v4"
)

type MailerConfig struct {
	Domain      string
	APIKey      string
	SenderName  string
	SenderEmail string
}

type Mailer struct {
	mg          *mailgun.MailgunImpl
	senderName  string
	senderEmail string
}

func NewMailer(mailerConfig MailerConfig) *Mailer {
	mg := mailgun.NewMailgun(mailerConfig.Domain, mailerConfig.APIKey)

	return &Mailer{
		mg:          mg,
		senderName:  mailerConfig.SenderName,
		senderEmail: mailerConfig.SenderEmail,
	}
}

func (m *Mailer) buildSender() string {
	return fmt.Sprintf("%s <%s>", m.senderName, m.senderEmail)
}

func (m *Mailer) SendPaymentConfirmationAndTicketReservation(recipientName string, recipientEmail string, eventDetailsWithEventTitle []database.GethEventDetailsWithTitleByIdsRow) error {
	eventConcat := ""
	for _, eventDetail := range eventDetailsWithEventTitle {
		eventConcat += fmt.Sprintf(`%s - %s - %s
`, eventDetail.Title, eventDetail.TicketDescription, eventDetail.ShowDate)
	}

	mailgunMessage := mailgun.NewMessage(
		m.buildSender(),
		"Your payment and ticket reservation are confirmed",
		fmt.Sprintf(`Hi %s,

Thank you for your payment and ticket reservation. Below are your event details.

%s

- Event - MRS Team`, recipientName, eventConcat),
		fmt.Sprintf("%s <%s>", recipientName, recipientEmail),
	)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second * 30)
	defer cancel()

	sendMessage, id, sendError := m.mg.Send(ctx, mailgunMessage)

	if sendError != nil {
		log.Printf("Mailgun error | Sender: %s <%s> | Recipient: %s <%s> | ID: %s | Message: %s | Error: %s", m.senderName, m.senderEmail, recipientName, recipientEmail, id, sendMessage, sendError)
		return fmt.Errorf("sender: %s <%s> | recipient: %s <%s> | ID: %s | message: %s | error: %s", m.senderName, m.senderEmail, recipientName, recipientEmail, id, sendMessage, sendError)
	}

	return nil
}

func (m *Mailer) SendRefundOrCancelledEmail(recipientName string, recipientEmail string, eventTitle string, refundOrCancelledNotifMessage string) error {
	mailgunMessage := mailgun.NewMessage(
		m.buildSender(),
		"Your booked event was cancelled and/or refunded",
		refundOrCancelledNotifMessage,
		fmt.Sprintf("%s <%s>", recipientName, recipientEmail),
	)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second * 30)
	defer cancel()

	sendMessage, id, sendError := m.mg.Send(ctx, mailgunMessage)

	if sendError != nil {
		log.Printf("Mailgun error | Sender: %s <%s> | Recipient: %s <%s> | ID: %s | Message: %s | Error: %s", m.senderName, m.senderEmail, recipientName, recipientEmail, id, sendMessage, sendError)
		return fmt.Errorf("sender: %s <%s> | recipient: %s <%s> | ID: %s | message: %s | error: %s", m.senderName, m.senderEmail, recipientName, recipientEmail, id, sendMessage, sendError)
	}

	return nil
}

func (m *Mailer) SendUpdatedEventNotification(recipientName string, recipientEmail string, eventTitle string, eventDescription string, eventOrganizer string) error {
	organizerText := ""
	if strings.TrimSpace(eventOrganizer) != "" {
		organizerText = fmt.Sprintf("Organizer: %s\r\n", eventOrganizer)
	}

	mailgunMessage := mailgun.NewMessage(
		m.buildSender(),
		"Your booked event was updated",
		fmt.Sprintf(`Hi %s,

You are receiving this email because your booked event has been updated. Please refer to details below.

Title: %s
Description: %s
%s
- Event - MRS Team`, recipientName, eventTitle, eventDescription, organizerText),
		fmt.Sprintf("%s <%s>", recipientName, recipientEmail),
	)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second * 30)
	defer cancel()

	sendMessage, id, sendError := m.mg.Send(ctx, mailgunMessage)
	
	if sendError != nil {
		log.Printf("Mailgun error | Sender: %s <%s> | Recipient: %s <%s> | ID: %s | Message: %s | Error: %s", m.senderName, m.senderEmail, recipientName, recipientEmail, id, sendMessage, sendError)
		return fmt.Errorf("sender: %s <%s> | recipient: %s <%s> | ID: %s | message: %s | error: %s", m.senderName, m.senderEmail, recipientName, recipientEmail, id, sendMessage, sendError)
	}

	return nil
}