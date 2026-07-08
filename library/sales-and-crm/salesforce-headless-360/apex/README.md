# SF360 Apex Companions

Deploy from this directory with:

```sh
sf project deploy start --target-org <alias>
```

The project contains three Apex REST companions:

- `SF360SafeRead`: `POST /services/apexrest/sf360/v1/safeRead` executes SELECT SOQL with `WITH USER_MODE` so Salesforce enforces the current user's sharing, CRUD, and FLS before records leave the org.
- `SF360SafeWrite`: `POST /services/apexrest/sf360/v1/safeWrite` accepts insert/update payloads and runs `Database.insert` or `Database.update` with `AccessLevel.USER_MODE`, returning D9-shaped write errors and an `fls_filtered` audit list.
- `SF360SafeUpsert`: `POST /services/apexrest/sf360/v1/safeUpsert` accepts External ID based upserts and runs `Database.upsert` with `AccessLevel.USER_MODE`, returning whether the write created a new record.
