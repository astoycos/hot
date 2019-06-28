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
	}
}
