# Spec Source Notes

The OpenAPI seed was derived from the official Cargo registry web API documentation. It covers the public crates.io read endpoints used for discovery and package analysis:

- `GET /api/v1/crates`
- `GET /api/v1/crates/{crate_name}`
- `GET /api/v1/crates/{crate_name}/versions`
- `GET /api/v1/crates/{crate_name}/{version}/dependencies`
- `GET /api/v1/crates/{crate_name}/owners`
- `GET /api/v1/crates/{crate_name}/reverse_dependencies`
- `GET /api/v1/crates/{crate_name}/downloads`

The seed excludes registry write endpoints because this library entry is read-only.
