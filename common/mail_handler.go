package common

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/elorenzorodz/event-mrs/internal/database"
	"github.com/mailgun/mailgun-go/v4"
)

func SendPaymentConfirmationAndTicketReservation(recipientName string, recipientEmail string, eventDetailsWithEventTitle []database.GethEventDetailsWithTitleByIdsRow) error {
	mailgunSendingDomain := GetEnvVariable("MAILGUN_SENDING_DOMAIN")
	mailgunAPIKey := GetEnvVariable("MAILGUN_API_KEY")

	senderName := GetEnvVariable("SENDER_NAME")
	senderEmail := GetEnvVariable("SENDER_EMAIL")

	eventConcat := ""

	for _, eventDetail := range eventDetailsWithEventTitle {
		eventConcat += fmt.Sprintf(`%s - %s - %s
`, eventDetail.Title, eventDetail.TicketDescription, eventDetail.ShowDate)
	}

	mg := mailgun.NewMailgun(mailgunSendingDomain, mailgunAPIKey)

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
	mailgunSendingDomain := GetEnvVariable("MAILGUN_SENDING_DOMAIN")
	mailgunAPIKey := GetEnvVariable("MAILGUN_API_KEY")

	senderName := GetEnvVariable("SENDER_NAME")
	senderEmail := GetEnvVariable("SENDER_EMAIL")

	mg := mailgun.NewMailgun(mailgunSendingDomain, mailgunAPIKey)

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