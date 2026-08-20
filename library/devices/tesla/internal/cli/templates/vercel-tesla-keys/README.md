# Tesla Fleet API public-key host

A minimal static site that hosts your EC P256 public key for Tesla Fleet API
partner-app registration.

## Deploy

Install the Vercel CLI once (`npm i -g vercel`), then from this directory:

```
vercel deploy --prod
```

Vercel prints a production hostname when the deploy finishes. That hostname is
the value to register as the partner-app domain at developer.tesla.com and to
pass to `tesla auth fleet-register --public-key-domain <hostname>`.

## Verify

After deploy, fetch your key to confirm it is reachable:

```
curl https://<your-hostname>/.well-known/appspecific/com.tesla.3p.public-key.pem
```

The response should start with `-----BEGIN PUBLIC KEY-----` and the content
type should be `application/x-pem-file`. Tesla scans this URL during partner
account registration, so it must be HTTPS with a valid certificate (Vercel
provides this automatically).

## Update the key

Replace `public/.well-known/appspecific/com.tesla.3p.public-key.pem` with your
real public key (the `tesla auth fleet-template --gen-key` command does this
for you), then redeploy.

## What about the private key?

It does NOT live here. The matching private key stays on the machine where
you sign Tesla commands, at `~/.tesla/<vehicle-or-app>-private.pem` with
mode 600. Never commit it.
