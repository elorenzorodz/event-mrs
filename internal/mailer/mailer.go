package mailer

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/elorenzorodz/event-mrs/internal/database"
	"github.com/mailgun/mailgun-go/v4"
)

type Mailer struct {
	mg          mailgun.Mailgun
	senderName  string
	senderEmail string
	teamName    string
}

func NewMailer(apiKey, domain, senderName, senderEmail, teamName string) *Mailer {
	mg := mailgun.NewMailgun(domain, apiKey)

	return &Mailer{
		mg:          mg,
		senderName:  senderName,
		senderEmail: senderEmail,
		teamName:    teamName,
	}
}

func (m *Mailer) SendPaymentConfirmationAndTicketReservation(recipientName string, recipientEmail string, eventDetailsWithEventTitle []database.GethEventDetailsWithTitleByIdsRow) error {

	eventConcat := ""
	for _, eventDetail := range eventDetailsWithEventTitle {
		// Assuming eventDetail.ShowDate is a string or compatible type
		eventConcat += fmt.Sprintf(`%s - %s - %s\r\n`, eventDetail.Title, eventDetail.TicketDescription, eventDetail.ShowDate)
	}

	mailgunMessage := m.mg.NewMessage(
		fmt.Sprintf("%s <%s>", m.senderName, m.senderEmail),
		"Your payment and ticket reservation confirmation",
		fmt.Sprintf(`Hi %s,\r\n\r\nThank you for your payment and reservation. Your tickets are confirmed.\r\n\r\nTickets:\r\n%s\r\n\r\n- %s Team`, recipientName, eventConcat, m.teamName),
		fmt.Sprintf("%s <%s>", recipientName, recipientEmail),
	)

	return m.send(mailgunMessage, recipientName, recipientEmail)
}

func (m *Mailer) SendUpdatedEventNotification(recipientName string, recipientEmail string, eventTitle string, eventDescription string, eventOrganizer string) error {
	organizer := ""
	if strings.TrimSpace(eventOrganizer) != "" {
		organizer = eventOrganizer
	}

	mailgunMessage := m.mg.NewMessage(
		fmt.Sprintf("%s <%s>", m.senderName, m.senderEmail),
		"Your booked event was updated",
		fmt.Sprintf(`Hi %s,\r\n\r\nYou are receiving this email because your booked event has been updated. Please refer to details below.\r\n\r\nTitle: %s\r\nDescription: %s\r\n%s\r\n\r\n- %s Team`, recipientName, eventTitle, eventDescription, organizer, m.teamName),
		fmt.Sprintf("%s <%s>", recipientName, recipientEmail),
	)

	return m.send(mailgunMessage, recipientName, recipientEmail)
}

func (m *Mailer) send(mailgunMessage *mailgun.Message, recipientName, recipientEmail string) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	sendMessage, id, sendError := m.mg.Send(ctx, mailgunMessage)

	if sendError != nil {
		// Log the full error internally
		return fmt.Errorf("mail send failed: sender: %s <%s> | recipient: %s <%s> | ID: %s | message: %s | error: %w", m.senderName, m.senderEmail, recipientName, recipientEmail, id, sendMessage, sendError)
	}

	return nil
}