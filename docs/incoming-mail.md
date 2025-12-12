# Incoming Mail Processing

This feature allows listmonk to receive emails and forward them to mailing list subscribers.

## Overview

The incoming mail processing feature enables you to:
1. Assign an incoming email address to each mailing list
2. Configure POP3 mailboxes to retrieve incoming emails
3. Automatically forward received emails to all subscribers of the target list
4. Preserve the original sender's address so replies go back to them

## Configuration

### 1. Enable Incoming Mail Processing

Add the following to your `config.toml`:

```toml
[incoming]
enabled = true

# Configure one or more POP3 mailboxes to scan for incoming emails
[[incoming.mailboxes]]
enabled = true
uuid = "unique-mailbox-uuid"
type = "pop"
host = "mail.example.com"
port = 995
auth_protocol = "plain"  # or "none"
username = "lists@example.com"
password = "your-password"
tls_enabled = true
tls_skip_verify = false
scan_interval = "5m"  # Scan every 5 minutes
```

### 2. Assign Incoming Email to Lists

When creating or updating a list via the API or admin UI, set the `incoming_email` field:

```json
{
  "name": "My List",
  "type": "public",
  "incoming_email": "mylist@example.com"
}
```

**Important**: 
- Each list can have only one incoming email address
- The incoming email address must be unique across all lists
- The email address must be deliverable to one of the configured POP3 mailboxes

## How It Works

1. The background service scans configured POP3 mailboxes at the specified interval
2. For each email found:
   - Checks if the recipient matches any list's `incoming_email`
   - Parses the email (from, subject, body)
   - Forwards it to all confirmed subscribers of that list
   - Uses the **original sender's address** as the FROM address
   - Deletes the processed email from the POP3 mailbox

3. When subscribers reply, the reply goes directly to the original sender (not back to the list)

## Email Forwarding Behavior

- **FROM Address**: Original sender's email (e.g., `sender@company.com`)
- **TO Address**: Individual subscriber's email
- **Subject**: Original email subject
- **Body**: Original email body (HTML or plain text)

This preserves the natural email conversation flow and doesn't require complex envelope manipulation.

## Use Cases

### Office Workflow Integration

As described in the feature request, this allows you to:
1. Open Outlook (or any email client)
2. Send an email to your list address (e.g., `newsletter@example.com`)
3. The email is automatically forwarded to all list subscribers
4. Replies come back to you (the original sender)

### Multiple Senders

Since the original sender's address is preserved:
- Different team members can send emails to the list
- Each sender's address is used as FROM
- DMARC, DKIM, and SPF checks work naturally for internal senders

## Database Schema

The feature adds an `incoming_email` field to the `lists` table:

```sql
ALTER TABLE lists ADD COLUMN incoming_email TEXT NULL;
CREATE UNIQUE INDEX idx_lists_incoming_email ON lists(incoming_email) 
  WHERE incoming_email IS NOT NULL;
```

To upgrade an existing database, run:
```bash
./listmonk --upgrade
```

## Security Considerations

1. **POP3 Credentials**: Store passwords securely in your config file
2. **TLS**: Always use TLS for POP3 connections (`tls_enabled = true`)
3. **Email Validation**: The system only forwards emails to lists with matching incoming_email
4. **Original Sender**: The FROM address is preserved, so ensure your SMTP allows this

## Troubleshooting

### Emails not being forwarded

1. Check that `incoming.enabled = true` in config
2. Verify POP3 mailbox credentials are correct
3. Ensure the list has an `incoming_email` set
4. Check that emails are actually arriving in the POP3 mailbox
5. Review logs for any error messages

### Subscribers not receiving emails

1. Verify subscribers are in "confirmed" status
2. Check that the list is "active"
3. Review SMTP logs for delivery issues
4. Ensure your SMTP server allows using arbitrary FROM addresses

## Limitations

- Only POP3 is supported (not IMAP)
- The entire email body is forwarded as-is (no modification or templating)
- Attachments are not currently supported
- Each email is processed individually (not batched)
