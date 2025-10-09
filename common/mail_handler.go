package common

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/mailgun/mailgun-go/v4"
)

func SendRefundOrCancelledEmail(recipientName string, recipientEmail string, eventTitle string) error {
	mailgunSendingDomain := GetEnvVariable("")
	mailgunAPIKey := GetEnvVariable("")

	senderName := GetEnvVariable("")
	senderEmail := GetEnvVariable("")

	mg := mailgun.NewMailgun(mailgunSendingDomain, mailgunAPIKey)

	mailgunMessage := mailgun.NewMessage(
		fmt.Sprintf("%s <%s>", senderName, senderEmail),
		fmt.Sprintf("Your payment for %s was refunded/cancelled", eventTitle),
		fmt.Sprintf(`Hi %s, \n\nThe event: %s, that you booked was cancelled and your payment for the refunded. 
		If you didn't pay yet, the pending payment is now cancelled.`, recipientName, eventTitle),
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