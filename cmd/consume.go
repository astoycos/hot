/*******************************************************************************
 * Copyright (c) 2019 Red Hat Inc
 *
 * See the NOTICE file(s) distributed with this work for additional
 * information regarding copyright ownership.
 *
 * This program and the accompanying materials are made available under the
 * terms of the Eclipse Public License 2.0 which is available at
 * http://www.eclipse.org/legal/epl-2.0
 *
 * SPDX-License-Identifier: EPL-2.0
 *******************************************************************************/

package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"

	"github.com/ctron/hot/pkg/utils"
	"pack.ag/amqp"
)

func consume(messageType string, uri string, tenant string) error {

	fmt.Printf("Consuming %s from %s ...", messageType, uri)
	fmt.Println()

	opts := make([]amqp.ConnOption, 0)
	if insecure {
		opts = append(opts, amqp.ConnTLSConfig(createTlsConfig()))
	}

	client, err := amqp.Dial(uri, opts...)
	if err != nil {
		return err
	}

	defer func() {
		if err := client.Close(); err != nil {
			log.Fatal("Failed to close client:", err)
		}
	}()

	var ctx = context.Background()

	session, err := client.NewSession()
	if err != nil {
		return err
	}

	defer func() {
		if err := session.Close(ctx); err != nil {
			log.Fatal("Failed to close session:", err)
		}
	}()

	receiver, err := session.NewReceiver(
		amqp.LinkSourceAddress(fmt.Sprintf("%s/%s", messageType, tenant)),
		amqp.LinkCredit(10),
	)
	if err != nil {
		return err
	}
	defer func() {
		ctx, cancel := context.WithTimeout(ctx, 1*time.Second)
		if err := receiver.Close(ctx); err != nil {
			log.Fatal("Failed to close receiver: ", err)
		}
		cancel()
	}()

	fmt.Printf("Consumer running, press Ctrl+C to stop...")
	fmt.Println()

	for {
		// Receive next message
		msg, err := receiver.Receive(ctx)
		if err != nil {
			return err
		}

		// Accept message
		if err := msg.Accept(); err != nil {
			return nil
		}

		utils.PrintMessage(msg)
		if processCommands {
			if err := processCommand(session, tenant, msg); err != nil {
				log.Print("Failed to send command: ", err)
			}
		}
	}
}

func processCommand(session *amqp.Session, tenant string, msg *amqp.Message) error {
	ttd, ok := msg.ApplicationProperties["ttd"].(int32)

	if !ok {
		return nil
	}

	if ttd < 0 {
		return nil
	}

	deviceId, ok := msg.Annotations["device_id"].(string)
	if !ok || deviceId == "" {
		return nil
	}

	reader := &StdinCommandReader{}

	fmt.Printf("Enter command response (%v s): ", ttd)

	cmd := reader.ReadCommand(time.Duration(ttd) * time.Second)

	if cmd == nil {
		fmt.Print("Timeout!")
		fmt.Println()
		return nil
	}

	// open sender

	sender, err := session.NewSender(
		amqp.LinkTargetAddress("control/" + tenant + "/" + deviceId),
	)

	if err != nil {
		return err
	}

	// defer: close sender

	defer func() {
		if err := sender.Close(context.Background()); err != nil {
			log.Print("Failed to close sender: ", err)
		}
	}()

	// prepare message

	send := amqp.NewMessage([]byte(*cmd))
	send.Properties = &amqp.MessageProperties{
		Subject: "CMD",
		To:      "control/" + tenant + "/" + deviceId,
	}

	// set message id

	id, err := uuid.NewRandom()
	if err != nil {
		return err
	}
	send.Properties.MessageID = amqp.UUID(id).String()

	// send message

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := sender.Send(ctx, send); err != nil {
		return err
	}

	fmt.Println("Command delivered!")

	return nil
}
