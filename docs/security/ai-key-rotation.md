# AI Provider Key Rotation Runbook

## Scope

This runbook covers database-managed AI provider keys for AI Content.

## Required Secret

Set `SECRET_AI__API_KEY_ENCRYPTION_SECRET` or `AI_API_KEY_ENCRYPTION_SECRET` to a strong secret of at least 32 characters before using database-managed AI keys.

Do not rotate this encryption secret until all encrypted records have been re-encrypted with the new secret in a controlled migration.

## Normal Rotation

1. Create a new provider key in the Admin AI Keys screen.
2. Let the backend test the new key before saving it.
3. Select the new key as active.
4. Generate or refine a small test post to confirm runtime usage.
5. Delete or deactivate the old key.
6. Revoke the old key in the provider console.
7. Review logs for `ai_content: api key audit` events.

## Incident Rotation

1. Revoke the suspected provider key in the provider console first.
2. Delete or deactivate the key in Admin AI Keys.
3. Create and select a new key.
4. Check provider billing and usage logs.
5. Search backend logs for the key fingerprint shown in audit logs.

## Security Guarantees

Stored records use AES-256-GCM ciphertext, nonce, version, HMAC fingerprint, and masked preview. Raw provider keys are only kept in memory long enough to test or call the provider and are never returned by the API.
