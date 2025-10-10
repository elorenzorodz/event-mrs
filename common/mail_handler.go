package common

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/elorenzorodz/event-mrs/internal/database"
	"github.com/mailgun/mailgun-go/v4"
)

var mailgunSendingDomain = GetEnvVariable("MAILGUN_SENDING_DOMAIN")
var mailgunAPIKey = GetEnvVariable("MAILGUN_API_KEY")

var senderName = GetEnvVariable("SENDER_NAME")
var senderEmail = GetEnvVariable("SENDER_EMAIL")

var mg = mailgun.NewMailgun(mailgunSendingDomain, mailgunAPIKey)

func SendPaymentConfirmationAndTicketReservation(recipientName string, recipientEmail string, eventDetailsWithEventTitle []database.GethEventDetailsWithTitleByIdsRow) error {
	eventConcat := ""

	for _, eventDetail := range eventDetailsWithEventTitle {
		eventConcat += fmt.Sprintf(`%s - %s - %s
`, eventDetail.Title, eventDetail.TicketDescription, eventDetail.ShowDate)
	}

	mailgunMessage := mailgun.NewMessage(
		fmt.Sprintf("%s <%s>", senderName, senderEmail),
		"Your payment and ticket reservation is confirmed",
		fmt.Sprintf(`Hi %s,
You've successfully booked your events. Enjoy!
%s

- Event - MRS Team`, recipientName, eventConcat),
		fmt.Sprintf("%s <%s>", recipientName, recipientEmail),
	)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second * 30)
	defer cancel()

	sendMessage, id, sendError := mg.Send(ctx, mailgunMessage)
	
	if sendError != nil {
		log.Printf("Mailgun send error | ID: %s | Message: %s | Error: %s", id, sendMessage, sendError)

		return fmt.Errorf("sender: %s <%s> | recipient: %s <%s> | ID: %s | message: %s | error: %s", senderName, senderEmail, recipientName, recipientEmail, id, sendMessage, sendError)
	}

	return nil
}

func SendRefundOrCancelledEmail(recipientName string, recipientEmail string, eventTitle string) error {
	mailgunMessage := mailgun.NewMessage(
		fmt.Sprintf("%s <%s>", senderName, senderEmail),
		fmt.Sprintf("Your payment for %s was refunded/cancelled", eventTitle),
		fmt.Sprintf(`Hi %s,
The event: %s, that you booked was cancelled and your payment was refunded. 
If you didn't pay yet, the pending payment is now cancelled.
Sorry for the inconvencience.

- Event - MRS Team`, recipientName, eventTitle),
		fmt.Sprintf("%s <%s>", recipientName, recipientEmail),
	)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second * 30)
	defer cancel()

	sendMessage, id, sendError := mg.Send(ctx, mailgunMessage)
	
	if sendError != nil {
		log.Printf("Mailgun send error | ID: %s | Message: %s | Error: %s", id, sendMessage, sendError)

		return fmt.Errorf("sender: %s <%s> | recipient: %s <%s> | ID: %s | message: %s | error: %s", senderName, senderEmail, recipientName, recipientEmail, id, sendMessage, sendError)
	}

	return nil
}

func SendRefundErrorNotification() error {
	teamName := GetEnvVariable("TEAM_NAME")
	teamEmail := GetEnvVariable("TEAM_EMAIL")

	mailgunMessage := mailgun.NewMessage(
		fmt.Sprintf("%s <%s>", senderName, senderEmail),
		"A refund request has failed",
		"A refund request has failed. Please check logs.",
		fmt.Sprintf("%s <%s>", teamName, teamEmail),
	)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second * 30)
	defer cancel()

	sendMessage, id, sendError := mg.Send(ctx, mailgunMessage)
	
	if sendError != nil {
		log.Printf("Mailgun send error | ID: %s | Message: %s | Error: %s", id, sendMessage, sendError)

		return fmt.Errorf("sender: %s <%s> | recipient: %s <%s> | ID: %s | message: %s | error: %s", senderName, senderEmail, teamName, teamEmail, id, sendMessage, sendError)
	}

	return nil
}