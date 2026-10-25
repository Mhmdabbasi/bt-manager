# Phase 0 — Domain foundation and API contracts

Status: additive SQLite schema foundation, read-only RouterOS profile imports, and the initial card-package catalog API/UI are implemented; remaining contracts in this document are planned.
Branch: `feature/phase-1-foundation`.

## Goals

1. Separate RouterOS technical configuration from the commercial card package.
2. Represent HotSpot and User Manager profiles without assuming their properties or API paths are identical.
3. Model User Manager profile-to-limitation relationships as many-to-many.
4. Treat an approved point of sale as a distributor that can receive card batches, not as a synonym for direct retail checkout.
5. Persist real template assets and versioned design/page-layout snapshots, so templates can be reopened and batches can be reprinted using their original layout.
6. Add new relationships without deleting or silently reclassifying existing vouchers, voucher products, receipts, or financial transactions.

## Schema added in migrations 0005–0010

### Source and service data

- `routers` stores a local router reference and non-secret identifying details. Router credentials are deliberately not part of this table.
- `service_profiles` provides stable local IDs for imported HotSpot and User Manager profiles. `source_system` is either `hotspot` or `usermanager`; `source_id` retains the identifier returned by RouterOS; `properties_json` preserves version-specific properties. Syncs should upsert rows and mark missing rows as not present instead of deleting referenced rows.
- `um_limitations` stores User Manager limitations.
- `um_profile_limitations` stores the many-to-many relation, including its order and any source relationship metadata. A SQLite trigger requires the profile to be a User Manager profile and both records to belong to the same router.
- `card_packages` represents the commercial product. It references one technical profile, stores user and dealer price separately in minor currency units, the package-level prefix, generation options, and an active flag. A trigger prevents a HotSpot package from referring to a User Manager profile or vice versa.

A profile can disappear from the router during a sync without destroying package or batch history. Domain services must prevent generating new vouchers from profiles that are no longer present until the user explicitly resolves the condition.

### Authorized points of sale and template selection

- `points_of_sale` represents a grocery, office, or other approved distributor. Its default `package_selection_mode='all'` means every active package is available unless an explicit per-package rule disables it. In `selected` mode only packages with an enabled rule are available.
- `point_of_sale_packages` stores per-package enabled state, optional prefix override, and optional template override. A NULL prefix uses `card_packages.default_prefix`; a NULL template override falls back to the package's active global default template.
- `voucher_stock_movements` is an inventory event ledger for issue, allocation, transfer, return, sale, void, and adjustment. Printing is not a sale, and allocation to a distributor is not evidence of payment. Financial settlement must remain a separate concern.

Application-level validation must confirm that an overridden template belongs to the selected package and is either global or scoped to the same point of sale. This relationship is additionally protected by SQLite triggers.

### Card design and output

- `design_assets` stores asset metadata and a generated storage key. Binary files live in the application's managed data directory, not inside SQLite and not in a web URL supplied by the caller.
- `card_templates` scopes a template to a package and either globally or to a specific point of sale. Multiple templates can be saved. Only one active global default template per package may be selected.
- `card_template_versions` stores immutable versions with card width/height in millimetres, design-element JSON, page-layout JSON, and an optional background asset. Changes create a new version; update/delete triggers protect older versions. A print job/batch stores the version ID used so later edits do not change old batch reprints.
- `print_jobs` records each initial print or reprint separately, using the exact immutable template version linked to the batch and a safe relative output key. It records output status, page/card counts, errors, and timestamps; it does not create vouchers or imply a sale.

Recommended design JSON is a versioned document, e.g.:
```json
{
  "schemaVersion": 1,
  "elements": [
    {
      "id": "username",
      "kind": "variable",
      "field": "username",
      "visible": true,
      "xMm": 8,
      "yMm": 6,
      "widthMm": 34,
      "heightMm": 5,
      "fontFamily": "Tajawal",
      "fontSizePt": 10,
      "fontWeight": 700,
      "color": "#13283F",
      "align": "center",
      "rotationDeg": 0
    }
  ]
}
```

Element coordinates are measured from the top-left edge of the card in millimetres. The rendering engine must use the same geometry/font settings for canvas preview and PDF. The page layout is a separate versioned document containing paper size, rows/columns, margins, gaps, orientation, and print scale. The editor must validate that the layout fits the requested paper and must not silently stretch assets or text.

Only raster background formats PNG/JPEG/WebP are allowed at the initial stage. Upload processing must check the actual file signature, enforce a size limit, generate a safe storage key, calculate SHA-256 and dimensions, and prevent path traversal. Never use the original file name as a filesystem path.

### Additive connections to existing data

Migration 0007 adds nullable links/snapshots to `voucher_batches` and nullable package/POS links to `vouchers`. Existing records stay readable; old rows retain NULL links and are not assigned a package or distributor by guessing profile names. Existing `voucher_products`, `pos_sales`, `pos_sale_items`, and `financial_transactions` remain unchanged. The existing direct-to-customer POS flow is not deleted or implicitly reinterpreted as distributor inventory.

For new batches, store package name, service system, user price, dealer price, prefix/generation settings, selected POS, and template version as a creation-time snapshot. The template version row itself is immutable. Keep an explicit inventory movement for each assignment or transfer.

## API contracts and implementation status

All requests remain local to the application. Router credentials are passed only to the operation that requires router access, never returned by list endpoints or persisted in logs. Endpoints below are marked as implemented or planned so the UI does not rely on a contract that is not available yet.

### User Manager profile import — read-only implemented

- `POST /api/router/usermanager/sync`
  - Request: `{ "router": { "address": "...", "port": 8728, "username": "...", "password": "...", "useTls": false } }`.
  - Reads the RouterOS version first, then reads profiles, limitations, and profile-limitation relationships using version-specific paths. It performs no User Manager writes.
  - Response: router and sync metadata, imported profile rows with their currently related limitations, plus a standalone list of all limitations, including limitations not assigned to a profile.
  - The request's router password is transient: it is neither stored in the router table nor returned in the response.
  - The local sync is committed only after all three RouterOS read operations succeed and the relationships validate. A failed or incomplete response leaves the prior imported snapshot intact.
- `GET /api/usermanager/profiles?routerId={id}`
  - Returns the saved profiles and their last-known relationships, plus the standalone limitation inventory. Rows that were absent from a later successful router sync are marked `present=false` instead of being deleted.
- RouterOS v6 uses the legacy `/tool/user-manager/profile` menu, including `/tool/user-manager/profile/limitation` and `/tool/user-manager/profile/profile-limitation`. RouterOS v7 uses `/user-manager/profile`, `/user-manager/limitation`, and `/user-manager/profile-limitation`.
- If the version is unsupported, or reading any required User Manager menu fails, the API returns an explicit error instead of reporting a successful empty sync.

### HotSpot profile catalog — read-only implemented

- `POST /api/router/hotspot/profiles/sync` reads all HotSpot user profiles and upserts them into the same `service_profiles` catalog under `source_system='hotspot'`.
- `GET /api/service-profiles?routerId={id}&system={hotspot|usermanager}&includeMissing={boolean}` returns source-specific profiles with stable local IDs, RouterOS source IDs, properties, presence state, and last-seen time.
- Both HotSpot and User Manager snapshots use the same router identity key (address, API port, and TLS mode), allowing a commercial card package to refer to a service profile in either system without conflating similarly named profiles.
- HotSpot profile syncing is read-only and does not modify the router. A failed read does not update the local profile catalog; records absent from a successful sync are marked missing instead of deleted.

The shared service-profile API currently supports reading/synchronizing HotSpot profiles and reading the User Manager profiles persisted by the User Manager sync. User Manager profile-to-limitation details are returned by `/api/usermanager/profiles` and the User Manager sync endpoint.

### Card packages and points of sale

**Implemented — card package catalog**

- `GET /api/card-packages?includeInactive={boolean}&system={hotspot|usermanager}` lists packages, optionally including inactive packages and filtering by source system.
- `POST /api/card-packages` creates a package. `GET /api/card-packages/{id}` reads one package and `PUT /api/card-packages/{id}` updates it.
- `POST /api/card-packages/{id}/archive` deactivates a package without deleting it.
- The local **Card Package Catalog** UI links a commercial package to an imported HotSpot or User Manager service profile and records description, customer price, dealer price, default username prefix, active state, and display order.
- The store verifies that a selected profile belongs to the chosen service system. A new package must use a currently present profile; an existing package whose profile disappears is retained and marked as missing. It cannot be activated again until that profile is present.
- Package names are checked without case sensitivity. Prices use integer minor currency units; dealer price cannot be negative or exceed customer price. Package records are archived rather than deleted.

**Implemented — points of sale and package distribution rules**

- `GET /api/points-of-sale?includeInactive={boolean}` lists points of sale; `POST /api/points-of-sale` creates one.
- `GET /api/points-of-sale/{id}` and `PUT /api/points-of-sale/{id}` read and update a point of sale.
- `POST /api/points-of-sale/{id}/archive` deactivates a point of sale without deleting its history.
- `GET /api/points-of-sale/{id}/packages?includeInactive={boolean}` lists the card packages and their effective permission for this point of sale.
- `PUT /api/points-of-sale/{id}/packages` atomically replaces package selection mode and package rules. Its request contains `packageSelectionMode` (`all` or `selected`) and a `packages` array. Each rule includes `packageId`, `isEnabled`, optional `prefixOverride`, and optional `templateIdOverride`.
- When mode is `all`, packages without an explicit rule are allowed by default; in `selected` mode, packages without a rule are denied. The store persists only explicit deviations from that default or actual overrides.
- Prefix overrides accept only ASCII letters, digits, underscores, and hyphens. Template overrides are checked against both package and point-of-sale scope, and the schema triggers independently enforce template scope.
- Point-of-sale codes are normalized to uppercase and must be unique. Point-of-sale records are deactivated rather than deleted.

The local **Points of Sale & Distributors** UI supports creating, updating, deactivating/reactivating distributors and configuring allowed packages. Saving assignments does not generate vouchers, allocate inventory, or record a sale.

**Implemented — initial allocation during package-backed generation**

Package-backed HotSpot batch generation now consumes these rules. A successful batch writes an `issue` stock movement for the generated cards, then an `allocate` movement to the selected point of sale when one was chosen. The batch finalization and initial inventory movements commit atomically. The earlier direct retail POS flow remains separate and does not get converted into distributor inventory.

**Still planned — further inventory operations and durable printing**

Manual allocation of previously generated stock, transfer between points of sale, returns, adjustments, reconciliation views, durable template rendering, and print-job workflows still need their own APIs and UI. A generated batch is not a financial settlement, and initial allocation does not imply that the distributor has paid.

Use stable IDs in requests. Archive rather than delete records that have been referenced by a batch or movement.

### Assets and design studio

- `POST /api/design-assets` accepts multipart upload of an allowed raster image and returns asset metadata plus a local asset ID, never an arbitrary filesystem path.
- `GET /api/design-assets/{id}` streams a validated stored asset.
- `GET/POST /api/card-templates`, `GET/PUT /api/card-templates/{id}`, and `POST /api/card-templates/{id}/versions`.
- `GET /api/card-templates/{id}/versions` returns immutable version history.
- A version request includes card size in mm, versioned design JSON, page-layout JSON, and optional background asset ID. Server-side validation rejects unknown schema versions, invalid fields, non-finite coordinates, and assets not owned by the local application.

### Batch generation and printing

**Implemented — package-backed HotSpot generation**

- `POST /api/voucher-batches` accepts transient router credentials, `packageId`, `count`, optional `pointOfSaleId`, generation options, and the explicit `allowWithoutTemplate` policy flag.
- Generation is allowed only for an active HotSpot package whose imported profile is still present and whose saved router identity matches the request's address, API port, and TLS mode. User Manager packages return HTTP 422 until a separate User Manager write adapter exists; no write request is sent to the router for that unsupported path.
- If a point of sale is specified, it must be active and its package-selection rules must allow the package. A point-of-sale prefix override takes precedence over a one-off prefix; otherwise the submitted prefix falls back to the package's default prefix.
- Before any router writes, the API resolves the latest saved active template version. If none exists, generation is blocked unless `allowWithoutTemplate=true` is explicitly selected in the Voucher Studio. That fallback is designed for the existing browser preview/print flow and does not create a durable print-template version.
- The batch stores router/package/POS/template IDs when present, package name and commercial price snapshots, credential-generation settings, and a copy of the imported service-profile properties. Vouchers copy package/POS assignments from their batch. Batch completion appends an `issue` stock movement and, when a distributor was selected, an `allocate` movement in the same transaction that finalizes the batch.
- The Voucher Studio can select a package and optional point of sale. The existing `/api/router/hotspot/vouchers/generate` endpoint remains available for direct legacy generation.

**Planned — durable print jobs and User Manager writes**

- `POST /api/voucher-batches/{id}/print` will create a print job using the immutable template version. The print result will record status and page count; reprinting creates a separate print event and does not create or sell vouchers again.
- Do not route package-backed User Manager generation through the HotSpot writer. It requires its own adapter and tests before it can be enabled.

## Error and consistency semantics

- Invalid IDs/fields: HTTP 400.
- Missing referenced records: HTTP 404.
- Duplicate keys or conflicting state: HTTP 409.
- Unsupported RouterOS/User Manager capability or invalid template/package relationship: a typed client error (prefer HTTP 422 when the request is syntactically valid).
- A router write can partially succeed: return explicit batch status and per-item outcomes. Never retry an ambiguous write blindly.
- SQLite constraints/triggers are a backstop, not a replacement for user-friendly validation and audit logging.
- Financial settlement, printing, credential creation, and stock allocation are distinct events.

## Migration safety and completion checklist

- Migrations 0005–0010 are additive. Do not rewrite or discard earlier rows.
- No router password is stored in the new `routers` table.
- Do not auto-convert legacy `voucher_products` into `card_packages`; the old records lack a guaranteed stable profile/router relationship. Later, provide an explicit mapping flow if data migration is wanted.
- Keep legacy APIs/schema until the new package/profile paths are implemented and a migration plan is tested.
- Tests should verify migration count, FK integrity, profile/system checks, multi-limitation mappings, template scope, immutable template versions/assets, print-job references, package/POS snapshots, and legacy table survival.
