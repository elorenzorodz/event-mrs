package common

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/elorenzorodz/event-mrs/internal/database"
	"github.com/mailgun/mailgun-go/v4"
)

func initializeMailgun() (string, string, *mailgun.MailgunImpl){
	mailgunSendingDomain := GetEnvVariable("MAILGUN_SENDING_DOMAIN")
	mailgunAPIKey := GetEnvVariable("MAILGUN_API_KEY")

	senderName := GetEnvVariable("SENDER_NAME")
	senderEmail := GetEnvVariable("SENDER_EMAIL")

	mg := mailgun.NewMailgun(mailgunSendingDomain, mailgunAPIKey)

	return senderName, senderEmail, mg
}

func SendPaymentConfirmationAndTicketReservation(recipientName string, recipientEmail string, eventDetailsWithEventTitle []database.GethEventDetailsWithTitleByIdsRow) error {
	senderName, senderEmail, mg := initializeMailgun()
	
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

func SendRefundOrCancelledEmail(recipientName string, recipientEmail string, eventTitle string, message string) error {
	senderName, senderEmail, mg := initializeMailgun()

	mailgunMessage := mailgun.NewMessage(
		fmt.Sprintf("%s <%s>", senderName, senderEmail),
		fmt.Sprintf("Your payment for %s was refunded/cancelled", eventTitle),
		message,
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
	senderName, senderEmail, mg := initializeMailgun()

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

func SendPaymentFailedNotification(recipientName string, recipientEmail string, errorMessage string, eventDetailsWithEventTitle []database.GethEventDetailsWithTitleByIdsRow) error {
	senderName, senderEmail, mg := initializeMailgun()

	eventConcat := ""

	for _, eventDetail := range eventDetailsWithEventTitle {
		eventConcat += fmt.Sprintf(`%s - %s - %s
`, eventDetail.Title, eventDetail.TicketDescription, eventDetail.ShowDate)
	}

	mailgunMessage := mailgun.NewMessage(
		fmt.Sprintf("%s <%s>", senderName, senderEmail),
		"Your payment failed",
		fmt.Sprintf(`Hi %s,

Your payment failed with the following issue: %s

Reservation/s attached with the payment:
%s

- Event - MRS Team`, recipientName, errorMessage, eventConcat),
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

func SendUpdatedEventNotification(recipientName string, recipientEmail string, eventTitle string, eventDescription string, eventOrganizer string) error {
	senderName, senderEmail, mg := initializeMailgun()

	organizer := ""

	if strings.TrimSpace(eventOrganizer) != "" {
		organizer = eventOrganizer
	}

	mailgunMessage := mailgun.NewMessage(
		fmt.Sprintf("%s <%s>", senderName, senderEmail),
		"Your booked event was updated",
		fmt.Sprintf(`Hi %s,

You are receiving this email because your booked event has been updated. Please refer to details below.

Title: %s
Description: %s
%s

- Event - MRS Team`, recipientName, eventTitle, eventDescription, organizer),
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