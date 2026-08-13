import { SESv2Client, SendEmailCommand } from "@aws-sdk/client-sesv2";
import nodemailer, { type Transporter } from "nodemailer";

import type { AuthConfig } from "../config.js";

export interface EmailMessage {
  readonly to: string;
  readonly subject: string;
  readonly text: string;
  readonly html?: string;
}

export interface EmailProvider {
  send(message: EmailMessage): Promise<void>;
}

export class SmtpEmailProvider implements EmailProvider {
  readonly #transporter: Transporter;
  readonly #from: string;

  constructor(options: {
    readonly from: string;
    readonly host: string;
    readonly port: number;
    readonly secure: boolean;
  }) {
    this.#from = options.from;
    this.#transporter = nodemailer.createTransport({
      host: options.host,
      port: options.port,
      secure: options.secure,
      disableFileAccess: true,
      disableUrlAccess: true,
    });
  }

  async send(message: EmailMessage): Promise<void> {
    await this.#transporter.sendMail({
      from: this.#from,
      to: message.to,
      subject: message.subject,
      text: message.text,
      html: message.html,
    });
  }
}

export class SesEmailProvider implements EmailProvider {
  readonly #client: SESv2Client;
  readonly #from: string;

  constructor(options: { readonly from: string; readonly region: string }) {
    this.#from = options.from;
    this.#client = new SESv2Client({ region: options.region });
  }

  async send(message: EmailMessage): Promise<void> {
    await this.#client.send(
      new SendEmailCommand({
        FromEmailAddress: this.#from,
        Destination: { ToAddresses: [message.to] },
        Content: {
          Simple: {
            Subject: { Data: message.subject, Charset: "UTF-8" },
            Body: {
              Text: { Data: message.text, Charset: "UTF-8" },
              ...(message.html
                ? { Html: { Data: message.html, Charset: "UTF-8" } }
                : {}),
            },
          },
        },
      }),
    );
  }
}

export function createEmailProvider(configuration: AuthConfig["email"]): EmailProvider {
  return configuration.provider === "ses"
    ? new SesEmailProvider(configuration)
    : new SmtpEmailProvider(configuration);
}
