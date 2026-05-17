# Module Migration Waves

## Wave 1: Canonical Reference Modules

- `auth`
- `bot`
- `centers`
- `users`
- `lms`

These modules prove security-sensitive flows, ordinary CRUD, and a complex domain under the new standard.

## Wave 2: High-Value Business Modules

- `notifications`
- `documents`
- `posts`
- `profiles`
- `sessions`
- `recruitment`

These modules have meaningful business behavior and user-facing APIs but are less central than the first wave.

## Wave 3: Content and Operations Modules

- `crawler`
- `trending`
- `analytics`
- `audit`
- `seo`
- `reviews`
- `pages`
- `menus`
- `operations`

These modules benefit from standardization after the core patterns are proven.

## Wave 4: Lightweight or Specialized Modules

- `community`
- `courses`
- `elearning`
- `hoclieu`
- `public_disclosure`
- `sitemap`
- `bot`
- `ai_content`
- `access_control`

These modules should adopt the same dependency rules, but may remain lightweight where a full folder split adds no value yet.

## Migration Checklist

For each module:

1. identify the public API,
2. separate transport DTOs from domain/persistence types,
3. move business rules out of controllers,
4. isolate infrastructure adapters,
5. normalize exceptions and responses,
6. harden validation and form safety,
7. keep or add focused regression tests,
8. document any justified deviation from the standard.

## Current Migration Status

Completed on the modular layout:

- `access_control`
- `ai_content`
- `analytics`
- `audit`
- `auth`
- `bot`
- `centers`
- `community`
- `courses`
- `crawler`
- `documents`
- `elearning`
- `menus`
- `notifications`
- `operations`
- `pages`
- `posts`
- `profiles`
- `public_disclosure`
- `recruitment`
- `reviews`
- `seo`
- `sessions`
- `sitemap`
- `trending`
- `users`
- `hoclieu`
- `lms`

All modules now expose the canonical `api`, `application`, `domain`, and
`infrastructure` structure. Lightweight modules may keep marker packages until
there is enough behavior to justify additional files. `hoclieu` and `lms` keep
compatibility shims because existing tests and callers still depend on legacy
package-root contracts; those shims should be removed gradually when callers
import the layer-specific packages directly.
