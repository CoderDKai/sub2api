# Plan B Mailbox Access and Outlook Design

## Status

This document is the design spec for Plan B mailbox access only.
It covers the first detailed design slice for B1 Mailbox Access Layer and B2 Outlook Mailbox Access.
OpenAI account registration and card secret workflows remain separate follow-up topics.

## Context

- Plan B covers custom business features only.
- Mailbox access is a likely foundation for later OpenAI account registration work.
- The existing codebase already has system-level SMTP configuration, a general account management module, and a redeem code module, but it does not yet have a dedicated mailbox operations domain.
- The frontend currently uses explicit admin routes and sidebar menu entries rather than file-system routing.
- New work should follow the existing frontend and backend structure rather than introducing a separate `custom/` namespace.

## Goals

- Add an admin-only mailbox operations module under an independent admin menu group.
- Support mailbox access for SMTP, IMAP, POP3, and Outlook in a way that matches their real capability boundaries.
- Support read-only inbox browsing with scheduled header synchronization and on-demand body fetch.
- Support forwarded mailbox collection where the collector mailbox is not always the true business recipient.
- Support recipient resolution for two real source patterns:
  - catch-all style domain forwarding where multiple suffix rules may exist
  - imported iCloud private relay aliases that must be explicitly added or imported
- Keep the mailbox domain decoupled from later OpenAI registration orchestration.

## Non-Goals

- Building a full mail client.
- Sending, replying, deleting, or moving messages in the first version.
- Persisting full mailbox mirrors or permanent attachment archives.
- Opening mailbox management to normal users in the first version.
- Embedding B3 OpenAI account registration workflow into this design.
- Introducing a general scriptable rules engine in the first version.

## Decision Summary

### Scope order

- The first detailed Plan B design slice is B1 plus B2 together.
- Mailbox access is designed first because B3 is expected to consume mailbox capabilities rather than define them.

### Domain model direction

- Use a layered model, not a single flat mailbox record.
- The domain root is split into:
  - `ProviderAccount`
  - `CollectorMailbox`
  - `MailboxCapability`
  - `RecipientIdentity`
  - `RecipientMatchValue`
  - `MailHeaderCache`
  - `MailSyncJob`
- SMTP and IMAP/POP3 are different capabilities and should not be forced into one protocol config.
- Outlook import flow is treated as a Microsoft provider account, not as a generic mailbox protocol config.

### UI boundary

- The first version is admin-only.
- The frontend entry point is an independent admin menu group, not `Settings` and not the existing generic `Accounts` page.

### Inbox strategy

- Use scheduled synchronization of mail headers.
- Support manual immediate sync.
- Support batch sync for selected mailboxes or selected inbound capabilities.
- Fetch message body and attachment metadata on demand when the admin opens a detail view.

### Recipient resolution model

- The collector mailbox is not always the true recipient.
- The first version resolves messages to `RecipientIdentity`, not directly to a business account or workflow.
- `RecipientIdentity` can represent:
  - direct mailbox addresses
  - imported alias addresses such as iCloud relay addresses
  - catch-all forwarding identities backed by multiple domain suffix rules
- One recipient identity may have multiple exact addresses and multiple suffix rules.
- Exact imported aliases must match before general suffix rules.
- Suffix conflicts use longest-suffix-first, then explicit priority, then `ambiguous` if still unresolved.

## Architecture

## 1. Admin module boundaries

Add a new admin menu group: `Mailbox`.

The first version contains four pages:

- `Provider Accounts`
- `Collector Mailboxes`
- `Recipient Identities`
- `Inbox`

Responsibilities:

- `Provider Accounts` manages credential sources and provider-level validation.
- `Collector Mailboxes` manages actual inbound and outbound mailbox endpoints.
- `Recipient Identities` manages true recipient targets and matching values.
- `Inbox` provides read-only browsing over synchronized header data.

This keeps mailbox operations separate from:

- system-wide application settings
- AI provider account management
- later OpenAI registration flows

## 2. Core domain objects

### `ProviderAccount`

Represents a reusable credential source or provider identity.

Examples:

- Microsoft imported account for Outlook access
- Basic credential account for ordinary IMAP, POP3, or SMTP access

It stores:

- provider kind
- auth kind
- encrypted credentials
- provider-level status
- last validation result
- last validation time
- operational notes and metadata

It does not represent a mailbox by itself.

### `CollectorMailbox`

Represents the actual mailbox used for collection or delivery.

It stores:

- mailbox address
- display name
- business tags
- enabled state

This object represents the accessed mailbox endpoint, not necessarily the true business recipient.

Provider binding rule for the first version:

- `CollectorMailbox` is provider-agnostic and does not directly own provider credentials
- each `MailboxCapability` references exactly one `ProviderAccount`
- inbound and outbound capabilities on the same collector mailbox may reference different provider accounts when needed
- mailbox-level screens may still show inferred provider summaries derived from attached capabilities

### `MailboxCapability`

Represents a concrete mailbox capability attached to a collector mailbox.

First-version capability types:

- `outbound.smtp`
- `inbound.imap`
- `inbound.pop3`
- `inbound.microsoft_graph`

Each capability stores its own:

- connection settings
- linked provider account
- sync cursor or protocol cursor state
- health state
- last sync time
- last failure summary
- pause or enable state

This keeps SMTP and inbound protocols independently manageable while still belonging to the same collector mailbox.

### `RecipientIdentity`

Represents the true resolved recipient target for a message.

This is intentionally modeled at the email-address or alias level rather than at the business-account level.

Reasons:

- a normal mailbox may itself be the real recipient
- forwarded mailbox collectors may receive messages for many downstream targets
- Outlook often maps to multiple separate accounts
- later B3 workflows should consume recipient identities rather than define mailbox structure

### `RecipientMatchValue`

Represents one concrete matching value attached to a recipient identity.

First-version match types:

- `exact_address`
- `domain_suffix`

Each record stores:

- match type
- match value
- priority
- active state
- source metadata such as imported or manual
- notes

This supports:

- multiple imported iCloud aliases per identity
- multiple catch-all suffix rules per identity
- selective enable or disable of individual values

### `MailHeaderCache`

Stores synchronized header-level data used for inbox browsing.

It stores:

- source collector mailbox
- source capability
- remote message identifiers
- folder or mailbox path
- sender and recipient headers
- subject
- received time
- flags
- snippet
- envelope or original-recipient fields when available
- resolved recipient identity
- resolution source metadata
- `resolution_state`
- `detail_fetch_state`

It does not persist the full message body by default.

### `MailSyncJob`

Represents one sync execution for one inbound capability.

It stores:

- job state
- trigger source such as schedule or manual
- batch id for grouped manual actions
- target capability
- counts and timing
- retryability
- failure summary

Batch sync is implemented as many independent jobs sharing one batch id, not one giant job.

## 3. Outlook and ordinary mailbox handling

### Outlook

- Outlook access uses `ProviderAccount` with Microsoft provider semantics.
- Imported credential bundles are treated as provider credentials and stored encrypted.
- Outlook inbound read capability is modeled as `inbound.microsoft_graph`.
- Outlook should not be flattened into the same raw protocol config shape used for IMAP, POP3, or SMTP.

Outlook import contract for the first version:

- the import flow is an admin paste or text-file upload, not an interactive OAuth browser flow
- the first version supports one import format only: a plain-text Outlook import bundle string with four non-empty segments separated by `----`
- the segment contract is:
  - mailbox identifier
  - secret or password segment
  - provider account or client identifier segment
  - opaque Microsoft token bundle segment
- the first-version API contract accepts:
  - required `import_payload` string
  - optional `display_label`
  - optional `mailbox_hint`
- the backend stores the raw encrypted payload and any minimal parsed metadata it can safely derive for validation and display
- the backend must parse and validate the four-segment structure before storage
- the first version should extract and store at least:
  - mailbox identifier from segment one
  - provider account or client identifier from segment three
  - import timestamp
- if the payload does not match the supported import-bundle format, the system returns `invalid_format`
- if the payload parses but mailbox-read validation fails, the system returns `validation_failed`
- if the payload parses but the imported authorization is expired or revoked, the system returns `expired_or_revoked`
- validation success means the system can authenticate and perform a minimal mailbox-read probe through the intended Microsoft path
- if the imported credential later expires or becomes invalid, the provider account moves to invalid state and requires admin re-import or credential update
- the first version does not need generic token refresh orchestration across arbitrary import formats; it only needs explicit validation, invalid-state reporting, and re-import support

### Ordinary SMTP, IMAP, and POP3

- Basic credentials may back one or more mailbox capabilities.
- SMTP is outbound capability only.
- In the first version, SMTP scope is limited to configuration storage and outbound connectivity validation.
- The first version does not require operator-facing send workflows or SMTP-driven business jobs.
- IMAP and POP3 are inbound capabilities and keep their own independent health and sync state.
- A collector mailbox may have inbound only, outbound only, or both.

## 4. Recipient resolution model

Recipient resolution exists because the collector mailbox may act as a forwarding intake point rather than the real business recipient.

### Matching sources

Resolve from the best available recipient evidence in this order:

1. envelope recipient
2. `Delivered-To`
3. `X-Original-To`
4. `To`
5. `Cc`

### Matching phases

For each extracted candidate address:

1. match imported iCloud or other explicit relay aliases by `exact_address`
2. match ordinary explicit mailbox addresses by `exact_address`
3. match catch-all forwarding identities by `domain_suffix`
4. if more than one suffix matches, use longest suffix first
5. if still tied, use explicit priority
6. if still tied, mark the message `ambiguous`
7. if no rule matches, mark the message `unresolved`

Exact-address uniqueness rule:

- one active `exact_address` may belong to only one active recipient identity in the first version
- create, edit, and import flows must reject duplicate active exact addresses
- disabled exact-address records may coexist temporarily for audit history, but they cannot participate in matching
- this keeps exact-address resolution deterministic and avoids hidden ambiguity for imported relay aliases

Cross-field decision rule:

- candidate addresses are evaluated in source-field priority order
- the resolver stops at the first source field that yields at least one active match
- if that source field yields matches that all resolve to the same recipient identity, that identity wins
- if that source field yields matches to different recipient identities, the message is marked `ambiguous`
- lower-priority fields are ignored once a higher-priority field yields any match
- the stored `resolved recipient identity` in `MailHeaderCache` is therefore the single winner from the highest-priority matching source field, or empty when the result is `unresolved` or `ambiguous`

### Supported source patterns

#### Catch-all forwarding domains

- The system must support multiple suffix rules, not just one suffix.
- A recipient identity may own many suffix rules.
- This is the model for CF domain forwarding where prefixes may be unbounded and only suffix matching is stable.

#### iCloud private relay aliases

- These must be explicitly created or imported as exact addresses.
- They must not be inferred from suffix rules.
- This keeps imported relay addresses under direct administrative control.

### Resolution outputs

The system records:

- resolved recipient identity id
- resolved address
- match type
- matched value id
- resolution source field
- resolution confidence or ambiguity state

## 5. Frontend page behavior

### `Provider Accounts`

Supports:

- create provider account
- import Outlook credential bundles
- create basic credential providers
- test provider validation
- enable or disable provider accounts
- inspect validation summaries

### `Collector Mailboxes`

Supports:

- create collector mailbox
- attach inbound and outbound capabilities
- test capability connectivity
- enable or pause sync
- trigger immediate sync for one row
- trigger batch sync for selected rows or selected inbound capabilities

### `Recipient Identities`

Supports:

- create recipient identities
- add multiple exact addresses
- add multiple suffix rules
- bulk import exact addresses for iCloud relay cases
- enable or disable individual match values
- assign explicit priority to match values

### `Inbox`

Supports read-only browsing over local header cache.

Filters include:

- collector mailbox
- recipient identity
- folder
- date range
- resolution state
- detail fetch state

List rows show at least:

- subject
- from
- resolved recipient
- source mailbox
- received time
- flags
- resolution state

Opening a row:

- first shows cached header data
- then fetches full message body on demand
- may fetch attachment metadata on demand

The first version does not support send, reply, delete, or move actions.

## Data Flow

## 1. Provider and mailbox onboarding

### Ordinary mailbox onboarding

1. Admin creates or imports a basic provider account.
2. Admin validates the provider credentials.
3. Admin creates a collector mailbox.
4. Admin attaches IMAP, POP3, and optional SMTP capabilities as needed.
5. Admin tests capability connectivity.
6. Admin enables scheduled sync.

### Outlook onboarding

1. Admin creates a Microsoft provider account.
2. Admin imports the Outlook credential bundle.
3. Admin validates Microsoft access.
4. Admin creates a collector mailbox.
5. Admin attaches `inbound.microsoft_graph`.
6. Admin enables scheduled sync.

## 2. Scheduled synchronization

1. Scheduler scans enabled inbound capabilities.
2. One sync job is created per target capability.
3. The protocol adapter fetches incremental header data only.
4. New or changed messages are normalized into `MailHeaderCache`.
5. Recipient resolution runs for each synchronized message.
6. Resolved or unresolved state is persisted for inbox browsing.

First-version folder policy:

- IMAP and Microsoft Graph synchronize only the default Inbox in the first version
- POP3 is normalized to a fixed pseudo-folder value `INBOX`
- the UI `folder` filter therefore means normalized source folder, not arbitrary user-created mailbox hierarchy
- multi-folder synchronization is intentionally deferred to a later version

Protocol-specific cursor rules:

- IMAP uses protocol-safe incremental cursor state such as UID-based progress.
- POP3 uses dedupe-oriented cursor state because its protocol support is weaker.
- Microsoft Graph uses provider-appropriate incremental state such as provider delta or stable remote ids.

The framework is shared while protocol cursor details remain capability-specific.

Initial backfill boundary and retention rule for the first version:

- when a capability is synchronized for the first time, the system backfills only a bounded recent window
- default backfill target is the more conservative of:
  - the most recent 30 days
  - the most recent 500 messages in the normalized Inbox
- once the first bounded backfill is complete, later syncs are incremental only
- the first version does not require a retention-trimming job or rolling cleanup policy
- permanent historical mirroring remains out of scope because initial ingestion is bounded and later growth is incremental from that bounded starting point

## 3. Manual immediate sync

Admins may trigger sync from the collector mailbox page.

Manual sync modes:

- single mailbox sync
- batch sync for selected mailboxes
- batch sync for selected inbound capabilities

Execution rule:

- manual batch sync creates many independent `MailSyncJob` records with one shared `batch_id`
- one failing target must not block the others
- duplicate concurrent sync on the same capability must be prevented

## 4. Message detail fetch

1. Admin opens a cached message row in `Inbox`.
2. The UI shows local header data immediately.
3. Backend fetches body and attachment metadata from the source capability on demand.
4. Result is shown to the admin without turning the system into a full mailbox mirror.

Optional short-lived detail cache is allowed for the first version as an implementation optimization, but it is not required by the spec.

## Error Handling

## 1. Validation and connectivity failures

- Provider validation failures update provider status and summary.
- Capability connectivity failures update capability health independently.
- Repeated failures may move a capability to warning, error, or paused state.

## 2. Sync failures

- Retryable remote errors mark jobs as retryable and use backoff.
- One bad message must not fail the whole sync batch.
- A partial batch result is valid when some targets succeed and others fail.

## 3. Resolution failures

- If no recipient rule matches, mark the message `unresolved`.
- If multiple rules remain tied after suffix length and priority, mark it `ambiguous`.
- Resolution failure does not block inbox visibility.

## 4. Detail fetch failures

- Failure to fetch body or attachment metadata affects only the detail view.
- Header cache remains visible.

## State Model

### `ProviderAccount`

- `draft`
- `active`
- `invalid`
- `disabled`

### `MailboxCapability`

- `healthy`
- `warning`
- `error`
- `paused`
- `syncing`

### `MailSyncJob`

- `queued`
- `running`
- `succeeded`
- `partial`
- `failed`
- `cancelled`

### `MailHeaderCache`

`resolution_state`:

- `resolved`
- `unresolved`
- `ambiguous`

`detail_fetch_state`:

- `not_requested`
- `succeeded`
- `failed`

## Security and Audit

## 1. Sensitive credential handling

- Provider credentials must be stored encrypted.
- Full imported credential bundles, passwords, access tokens, or refresh tokens must never be logged.
- Admin UIs must not echo full sensitive values by default.

## 2. Mail content handling

- Full message bodies must not enter normal application logs.
- Attachments are not permanently mirrored by default.
- The first version caches headers only and fetches bodies on demand.

## 3. Required audit events

The first version must audit at least:

- create, update, disable, or delete provider account
- import Outlook credential bundle
- create or modify collector mailbox
- add or modify recipient identities or match values
- test validation or connectivity
- manual immediate sync
- batch sync trigger
- open message detail

Each event records:

- actor
- object type
- object id
- action
- result
- error summary if any
- timestamp

## Testing and Verification

## 1. Unit tests

Verify:

- recipient resolution priority order
- imported alias exact matching
- multi-suffix matching
- longest-suffix-first behavior
- tie-breaking by explicit priority
- `ambiguous` output when ties remain
- batch sync job splitting by batch id

## 2. Integration tests

Verify:

- provider account creation and validation
- collector mailbox creation
- capability connectivity checks
- incremental header sync and cursor persistence
- recipient resolution persistence into header cache
- on-demand message body fetch

## 3. Protocol adapter tests

Verify at minimum:

- IMAP incremental sync behavior
- POP3 dedupe behavior
- Outlook provider validation and inbound read flow

## 4. Frontend tests

Verify:

- mailbox admin pages load and filter correctly
- recipient identity editor supports multiple exact values and suffix values
- single and batch sync interactions work
- unresolved and ambiguous states are visible and understandable

## 5. Test account expectations

Implementation and integration testing will be stronger with fresh non-production test accounts.

Recommended test coverage matrix:

- one IMAP inbox with harmless seed messages
- one POP3 inbox with harmless seed messages
- one SMTP-capable outbound account for connectivity validation
- one Outlook or Microsoft account using the intended import flow
- one catch-all forwarding domain test setup with at least two configured suffix rules
- several imported iCloud relay aliases or equivalent exact-address test aliases

These accounts should be newly created and contain no business data.
Sensitive credentials should be provided only when implementation reaches integration testing, not embedded in the spec.

## Acceptance Criteria

- admins can create provider accounts for ordinary mailbox credentials and Outlook access
- admins can create collector mailboxes and attach inbound and outbound capabilities
- admins can configure multiple exact recipient addresses and multiple suffix rules
- scheduled sync pulls header data without requiring a full mailbox mirror
- admins can trigger immediate sync for one row or batch sync for selected rows
- synchronized messages resolve to the correct recipient identity, or clearly show `unresolved` or `ambiguous`
- inbox list browsing is stable from local cache and message detail can fetch body on demand
- sensitive credentials and message content are protected by audit and logging boundaries

## Operational Rules

- B1 and B2 are implemented as one mailbox-domain slice.
- B3 remains a downstream consumer of resolved recipient identities and mailbox content, not part of this first design.
- New mailbox pages should follow existing admin route and sidebar patterns.
- New mailbox code should prefer new files over broad modification of unrelated existing pages.
