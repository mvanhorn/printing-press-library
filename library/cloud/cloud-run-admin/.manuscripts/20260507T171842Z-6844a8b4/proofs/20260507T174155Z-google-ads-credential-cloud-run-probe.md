# Google Ads Credential Cloud Run Probe

Run ID: 20260507T171842Z-6844a8b4

## Purpose

Confirm whether the credentials previously used for the generated Google Ads CLI can also support Google Cloud Run Admin live checks.

## Credential Source

The local Google Ads CLI config at `~/.config/google-ads-pp-cli/config.toml` contains OAuth client and refresh-token material. The token values were not printed or committed. The refresh token successfully minted an access token with scope:

```text
https://www.googleapis.com/auth/cloud-platform
```

## Cloud Run Probe

`<cloud-run-disabled-project>` rejected Cloud Run Admin calls because the API is disabled or inaccessible in that project:

```text
HTTP 403: Cloud Run Admin API has not been used in project <cloud-run-disabled-project> before or it is disabled
```

The same credential source successfully listed Cloud Run services in `<cloud-run-enabled-project>`:

```bash
CLOUD_RUN_ADMIN_OAUTH2C="<cloud-platform token minted from Google Ads OAuth config>" \
./cloud-run-admin services list projects/<cloud-run-enabled-project>/locations/us-central1 --json --timeout 20s
```

Result: PASS. The output came from the live Cloud Run Admin API.

## Conclusion

The Google Ads OAuth credentials are usable for Cloud Run when minted with the Cloud Platform scope and pointed at a project where Cloud Run Admin API is enabled and accessible.
