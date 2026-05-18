---
name: pp-digitalocean
description: "Printing Press CLI for Digitalocean."
author: "Hiten Shah"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - digitalocean-pp-cli
    install:
      - kind: go
        bins: [digitalocean-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/cloud/digitalocean/cmd/digitalocean-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/cloud/digitalocean/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See AGENTS.md "Generated artifacts: registry.json, cli-skills/". -->

# Digitalocean — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `digitalocean-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install digitalocean --cli-only
   ```
2. Verify: `digitalocean-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/cloud/digitalocean/cmd/digitalocean-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`workflow archive`** — Sync DigitalOcean resources into a local SQLite store for offline inspection and repeatable agent workflows.

  _Gives agents a durable local inventory before planning cloud changes._

  ```bash
  digitalocean-pp-cli workflow archive --full --json
  ```
- **`search`** — Search synced DigitalOcean data locally, with live/local source selection for agent-friendly retrieval.

  _Lets agents locate relevant resources before choosing a narrower command._

  ```bash
  digitalocean-pp-cli search "production" --data-source local --json --limit 20
  ```

### Operational summaries
- **`analytics`** — Summarize locally synced resource data with count and group-by operations.

  _Turns resource listings into operational summaries an agent can reason about._

  ```bash
  digitalocean-pp-cli analytics --type droplets --group-by region --json
  ```
- **`tail`** — Poll selected resources and emit change events as NDJSON for shell and agent pipelines.

  _Provides a safe one-shot or continuous change feed without bespoke scripts._

  ```bash
  digitalocean-pp-cli tail --resource droplets --interval 30s --json
  ```

## Command Reference

**1-clicks** — Manage 1 clicks

- `digitalocean-pp-cli 1-clicks create` — Create
- `digitalocean-pp-cli 1-clicks list` — List

**account** — Provides information about your current account.

- `digitalocean-pp-cli account create` — Create
- `digitalocean-pp-cli account delete` — Delete
- `digitalocean-pp-cli account get` — Get
- `digitalocean-pp-cli account list` — List
- `digitalocean-pp-cli account list-keys` — List keys
- `digitalocean-pp-cli account update` — Update

**actions** — Actions are records of events that have occurred on the resources in your account.
These can be things like rebooting a Droplet, or transferring an image to a new region.

An action object is created every time one of these actions is initiated. The action
object contains information about the current status of the action, start and complete
timestamps, and the associated resource type and ID.

Every action that creates an action object is available through this endpoint. Completed
actions are not removed from this list and are always available for querying.

**Note:** You can pass the following HTTP header with the request to have the API return
the `reserved_ips` stanza instead of the `floating_ips` stanza:

- `Accept: application/vnd.digitalocean.reserveip+json`

- `digitalocean-pp-cli actions get` — Get
- `digitalocean-pp-cli actions list` — List

**add-ons** — Add-ons are third-party applications that can be added to your DigitalOcean account.
They are available through the [DigitalOcean Marketplace](https://marketplace.digitalocean.com/).
Add-ons can be used to enhance the functionality of your existing resources or to provide
additional services.

The Add-Ons API allows you to manage these resources, including creating, listing, and retrieving
details about specific add-on resources.

- `digitalocean-pp-cli add-ons create` — Create
- `digitalocean-pp-cli add-ons delete` — Delete
- `digitalocean-pp-cli add-ons get` — Get
- `digitalocean-pp-cli add-ons get-addons` — Get addons
- `digitalocean-pp-cli add-ons list` — List
- `digitalocean-pp-cli add-ons list-addons` — List addons
- `digitalocean-pp-cli add-ons update` — Update
- `digitalocean-pp-cli add-ons update-addons` — Update addons

**apps** — App Platform is a Platform-as-a-Service (PaaS) offering from DigitalOcean that allows
developers to publish code directly to DigitalOcean servers without worrying about the
underlying infrastructure.

Most API operations are centered around a few core object types. Following are the
definitions of these types. These definitions will be omitted from the operation-specific
documentation.

For documentation on app specifications (`AppSpec` objects), please refer to the
[product documentation](https://docs.digitalocean.com/products/app-platform/reference/app-spec/)).

- `digitalocean-pp-cli apps create` — Create
- `digitalocean-pp-cli apps create-metrics` — Create metrics
- `digitalocean-pp-cli apps create-propose` — Create propose
- `digitalocean-pp-cli apps delete` — Delete
- `digitalocean-pp-cli apps get` — Get
- `digitalocean-pp-cli apps get-tiers` — Get tiers
- `digitalocean-pp-cli apps list` — List
- `digitalocean-pp-cli apps list-regions` — List regions
- `digitalocean-pp-cli apps list-tiers` — List tiers
- `digitalocean-pp-cli apps update` — Update

**async-invoke** — Manage async invoke

- `digitalocean-pp-cli async-invoke` — Create

**batches** — Manage batches

- `digitalocean-pp-cli batches create` — Create
- `digitalocean-pp-cli batches create-files` — Create files
- `digitalocean-pp-cli batches get` — Get
- `digitalocean-pp-cli batches list` — List

**billing** — The billing endpoints allow you to retrieve your account balance, invoices,
billing history, and insights.

**Balance:** By sending requests to the `/v2/customers/my/balance` endpoint, you can
retrieve the balance information for the requested customer account.

**Invoices:** [Invoices](https://docs.digitalocean.com/platform/billing/invoices/)
are generated on the first of each month for every DigitalOcean
customer. An invoice preview is generated daily, which can be accessed
with the `preview` keyword in place of `$INVOICE_UUID`. To interact with
invoices, you will generally send requests to the invoices endpoint at
`/v2/customers/my/invoices`.

**Billing History:** Billing history is a record of billing events for your account.
For example, entries may include events like payments made, invoices
issued, or credits granted. To interact with invoices, you
will generally send requests to the invoices endpoint at
`/v2/customers/my/billing_history`.

**Billing Insights:** Day-over-day changes in billing resource usage based on nightly invoice items,
including total amount, region, SKU, and description for a specified date range.
It is important to note that the daily resource usage may not reflect month-end billing totals when totaled for
a given month as nightly invoice items do not necessarily encompass all invoicing factors for the entire month.
  `v2/billing/{account_urn}/insights/{start_date}/{end_date}` where account_urn is the URN of the customer
account, can be a team (do:team:uuid) or an organization (do:teamgroup:uuid). The date range specified by
start_date and end_date must be in YYYY-MM-DD format.


**byoip-prefixes** — Bring your own IP (BYOIP) lets you provision your own IPv4 network prefixes
to your account, then assign those IPs to your DigitalOcean resources.
BYOIP supports the following features:
* IPv4 addresses
* Network sizes of anywhere from `/24` (256 addresses) to `/18` (16,384 addresses)
* Same API and management interface as our existing reserved IPs feature
* Assignable to Droplets only

- `digitalocean-pp-cli byoip-prefixes create` — Create
- `digitalocean-pp-cli byoip-prefixes delete` — Delete
- `digitalocean-pp-cli byoip-prefixes get` — Get
- `digitalocean-pp-cli byoip-prefixes list` — List
- `digitalocean-pp-cli byoip-prefixes update` — Update

**cdn** — Manage cdn

- `digitalocean-pp-cli cdn create` — Create
- `digitalocean-pp-cli cdn delete` — Delete
- `digitalocean-pp-cli cdn delete-endpoints` — Delete endpoints
- `digitalocean-pp-cli cdn get` — Get
- `digitalocean-pp-cli cdn list` — List
- `digitalocean-pp-cli cdn update` — Update

**certificates** — In order to perform SSL termination on load balancers, DigitalOcean offers
two types of [SSL certificate management](https://docs.digitalocean.com/platform/teams/manage-certificates):

* **Custom**: User-generated certificates may be uploaded to DigitalOcean
where they will be placed in a fully encrypted and isolated storage system.

* **Let's Encrypt**: Certificates may be automatically generated by
DigitalOcean utilizing an integration with Let's Encrypt, the free and
open certificate authority. These certificates will also be automatically
renewed as required.

- `digitalocean-pp-cli certificates create` — Create
- `digitalocean-pp-cli certificates delete` — Delete
- `digitalocean-pp-cli certificates get` — Get
- `digitalocean-pp-cli certificates list` — List

**chat** — Manage chat

- `digitalocean-pp-cli chat create` — Create
- `digitalocean-pp-cli chat create-completions` — Create completions

**customers** — Manage customers

- `digitalocean-pp-cli customers get` — Get
- `digitalocean-pp-cli customers get-my` — Get my
- `digitalocean-pp-cli customers get-my-2` — Get my 2
- `digitalocean-pp-cli customers get-my-3` — Get my 3
- `digitalocean-pp-cli customers list` — List
- `digitalocean-pp-cli customers list-my` — List my
- `digitalocean-pp-cli customers list-my-2` — List my 2

**databases** — DigitalOcean's [managed database service](https://docs.digitalocean.com/products/databases)
simplifies the creation and management of highly available database clusters. Currently, it
offers support for [PostgreSQL](http://docs.digitalocean.com/products/databases/postgresql/),
[Caching](https://docs.digitalocean.com/products/databases/redis/),
[Valkey](https://docs.digitalocean.com/products/databases/valkey/),
[MySQL](https://docs.digitalocean.com/products/databases/mysql/),
[MongoDB](https://docs.digitalocean.com/products/databases/mongodb/), and
[OpenSearch](https://docs.digitalocean.com/products/databases/opensearch/).

By sending requests to the `/v2/databases` endpoint, you can list, create, or delete
database clusters as well as scale the size of a cluster, add or remove read-only replicas,
and manage other configuration details.

Database clusters may be deployed in a multi-node, high-availability configuration.
If your machine type is above the basic nodes, your node plan is above the smallest option,
or you are running MongoDB, you may additionally include up to two standby nodes in your cluster.

The size of individual nodes in a database cluster is represented by a human-readable slug,
which is used in some of the following requests. Each slug denotes the node's identifier,
CPU count, and amount of RAM, in that order.

For a list of currently available database slugs and options, use the `/v2/databases/options` endpoint or use the
`doctl databases options` [command](https://docs.digitalocean.com/reference/doctl/reference/databases/options).

- `digitalocean-pp-cli databases create` — Create
- `digitalocean-pp-cli databases delete` — Delete
- `digitalocean-pp-cli databases get` — Get
- `digitalocean-pp-cli databases list` — List
- `digitalocean-pp-cli databases list-metrics` — List metrics
- `digitalocean-pp-cli databases list-options` — List options
- `digitalocean-pp-cli databases update` — Update

**dedicated-inferences** — [Dedicated Inference](https://docs.digitalocean.com/products/agent-platform/dedicated-inference/)
delivers scalable production-grade LLM hosting on DigitalOcean. Create, list, get, update,
and delete Dedicated Inference instances; manage accelerators, CA certificate, sizes,
GPU model config, and access tokens.

- `digitalocean-pp-cli dedicated-inferences create` — Create
- `digitalocean-pp-cli dedicated-inferences delete` — Delete
- `digitalocean-pp-cli dedicated-inferences get` — Get
- `digitalocean-pp-cli dedicated-inferences list` — List
- `digitalocean-pp-cli dedicated-inferences list-dedicatedinferences` — List dedicatedinferences
- `digitalocean-pp-cli dedicated-inferences list-dedicatedinferences-2` — List dedicatedinferences 2
- `digitalocean-pp-cli dedicated-inferences update` — Update

**domains** — Domain resources are domain names that you have purchased from a domain
name registrar that you are managing through the
[DigitalOcean DNS interface](https://docs.digitalocean.com/products/networking/dns/).

This resource establishes top-level control over each domain. Actions that
affect individual domain records should be taken on the
[Domain Records](#tag/Domain-Records) resource.

- `digitalocean-pp-cli domains create` — Create
- `digitalocean-pp-cli domains delete` — Delete
- `digitalocean-pp-cli domains get` — Get
- `digitalocean-pp-cli domains list` — List

**droplets** — A [Droplet](https://docs.digitalocean.com/products/droplets/) is a DigitalOcean
virtual machine. By sending requests to the Droplet endpoint, you can
list, create, or delete Droplets.

Some of the attributes will have an object value. The `region` and `image`
objects will all contain the standard attributes of their associated
types. Find more information about each of these objects in their
respective sections.

- `digitalocean-pp-cli droplets create` — Create
- `digitalocean-pp-cli droplets create-actions` — Create actions
- `digitalocean-pp-cli droplets create-autoscale` — Create autoscale
- `digitalocean-pp-cli droplets delete` — Delete
- `digitalocean-pp-cli droplets delete-autoscale` — Delete autoscale
- `digitalocean-pp-cli droplets delete-autoscale-2` — Delete autoscale 2
- `digitalocean-pp-cli droplets delete-dropletid` — Delete dropletid
- `digitalocean-pp-cli droplets get` — Get
- `digitalocean-pp-cli droplets get-autoscale` — Get autoscale
- `digitalocean-pp-cli droplets get-autoscale-2` — Get autoscale 2
- `digitalocean-pp-cli droplets get-autoscale-3` — Get autoscale 3
- `digitalocean-pp-cli droplets list` — List
- `digitalocean-pp-cli droplets list-autoscale` — List autoscale
- `digitalocean-pp-cli droplets list-backups` — List backups
- `digitalocean-pp-cli droplets list-backups-2` — List backups 2
- `digitalocean-pp-cli droplets update` — Update

**embeddings** — Text embedding vectors via `POST /v1/embeddings` on the
[Serverless Inference](https://docs.digitalocean.com/reference/api/api-reference/#tag/Serverless-Inference) base URL
`https://inference.do-ai.run` (bearer model access key).

- `digitalocean-pp-cli embeddings` — Create

**firewalls** — [DigitalOcean Cloud Firewalls](https://docs.digitalocean.com/products/networking/firewalls/)
provide the ability to restrict network access to and from a Droplet
allowing you to define which ports will accept inbound or outbound
connections. By sending requests to the `/v2/firewalls` endpoint, you can
list, create, or delete firewalls as well as modify access rules.

- `digitalocean-pp-cli firewalls create` — Create
- `digitalocean-pp-cli firewalls delete` — Delete
- `digitalocean-pp-cli firewalls get` — Get
- `digitalocean-pp-cli firewalls list` — List
- `digitalocean-pp-cli firewalls update` — Update

**floating-ips** — As of 16 June 2022, we have renamed the Floating IP product to [Reserved IPs](https://docs.digitalocean.com/reference/api/api-reference/#tag/Reserved-IPs).
The Reserved IP product's endpoints function the exact same way as Floating IPs.
The only difference is the name change throughout the URLs and fields.
For example, the `floating_ips` field is now the `reserved_ips` field.
The Floating IP endpoints will remain active until fall 2023 before being
permanently deprecated.

With the exception of the [Projects API](https://docs.digitalocean.com/reference/api/api-reference/#tag/Projects),
we will reflect this change as an additional field in the responses across the API
where the `floating_ip` field is used. For example, the Droplet metadata response
will contain the field `reserved_ips` in addition to the `floating_ips` field.
Floating IPs retrieved using the Projects API will retain the original name.

[DigitalOcean Floating IPs](https://docs.digitalocean.com/products/networking/reserved-ips/)
are publicly-accessible static IP addresses that can be mapped to one of
your Droplets. They can be used to create highly available setups or other
configurations requiring movable addresses.

Floating IPs are bound to a specific region.

- `digitalocean-pp-cli floating-ips create` — Create
- `digitalocean-pp-cli floating-ips delete` — Delete
- `digitalocean-pp-cli floating-ips get` — Get
- `digitalocean-pp-cli floating-ips list` — List

**functions** — [Serverless functions](https://docs.digitalocean.com/products/functions) are blocks of code that run on demand without the need to manage any infrastructure.
You can develop functions on your local machine and then deploy them to a namespace using `doctl`, the [official DigitalOcean CLI tool](https://docs.digitalocean.com/reference/doctl).

The Serverless Functions API currently only supports creating and managing namespaces.

- `digitalocean-pp-cli functions create` — Create
- `digitalocean-pp-cli functions create-namespaces` — Create namespaces
- `digitalocean-pp-cli functions create-namespaces-2` — Create namespaces 2
- `digitalocean-pp-cli functions delete` — Delete
- `digitalocean-pp-cli functions delete-namespaces` — Delete namespaces
- `digitalocean-pp-cli functions delete-namespaces-2` — Delete namespaces 2
- `digitalocean-pp-cli functions get` — Get
- `digitalocean-pp-cli functions get-namespaces` — Get namespaces
- `digitalocean-pp-cli functions get-namespaces-2` — Get namespaces 2
- `digitalocean-pp-cli functions get-namespaces-3` — Get namespaces 3
- `digitalocean-pp-cli functions list` — List
- `digitalocean-pp-cli functions update` — Update
- `digitalocean-pp-cli functions update-namespaces` — Update namespaces

**gen-ai** — Manage gen ai

- `digitalocean-pp-cli gen-ai create` — Create
- `digitalocean-pp-cli gen-ai create-genai` — Create genai
- `digitalocean-pp-cli gen-ai create-genai-10` — Create genai 10
- `digitalocean-pp-cli gen-ai create-genai-11` — Create genai 11
- `digitalocean-pp-cli gen-ai create-genai-12` — Create genai 12
- `digitalocean-pp-cli gen-ai create-genai-13` — Create genai 13
- `digitalocean-pp-cli gen-ai create-genai-14` — Create genai 14
- `digitalocean-pp-cli gen-ai create-genai-15` — Create genai 15
- `digitalocean-pp-cli gen-ai create-genai-16` — Create genai 16
- `digitalocean-pp-cli gen-ai create-genai-17` — Create genai 17
- `digitalocean-pp-cli gen-ai create-genai-18` — Create genai 18
- `digitalocean-pp-cli gen-ai create-genai-19` — Create genai 19
- `digitalocean-pp-cli gen-ai create-genai-2` — Create genai 2
- `digitalocean-pp-cli gen-ai create-genai-20` — Create genai 20
- `digitalocean-pp-cli gen-ai create-genai-21` — Create genai 21
- `digitalocean-pp-cli gen-ai create-genai-22` — Create genai 22
- `digitalocean-pp-cli gen-ai create-genai-23` — Create genai 23
- `digitalocean-pp-cli gen-ai create-genai-24` — Create genai 24
- `digitalocean-pp-cli gen-ai create-genai-3` — Create genai 3
- `digitalocean-pp-cli gen-ai create-genai-4` — Create genai 4
- `digitalocean-pp-cli gen-ai create-genai-5` — Create genai 5
- `digitalocean-pp-cli gen-ai create-genai-6` — Create genai 6
- `digitalocean-pp-cli gen-ai create-genai-7` — Create genai 7
- `digitalocean-pp-cli gen-ai create-genai-8` — Create genai 8
- `digitalocean-pp-cli gen-ai create-genai-9` — Create genai 9
- `digitalocean-pp-cli gen-ai delete` — Delete
- `digitalocean-pp-cli gen-ai delete-genai` — Delete genai
- `digitalocean-pp-cli gen-ai delete-genai-10` — Delete genai 10
- `digitalocean-pp-cli gen-ai delete-genai-11` — Delete genai 11
- `digitalocean-pp-cli gen-ai delete-genai-12` — Delete genai 12
- `digitalocean-pp-cli gen-ai delete-genai-13` — Delete genai 13
- `digitalocean-pp-cli gen-ai delete-genai-2` — Delete genai 2
- `digitalocean-pp-cli gen-ai delete-genai-3` — Delete genai 3
- `digitalocean-pp-cli gen-ai delete-genai-4` — Delete genai 4
- `digitalocean-pp-cli gen-ai delete-genai-5` — Delete genai 5
- `digitalocean-pp-cli gen-ai delete-genai-6` — Delete genai 6
- `digitalocean-pp-cli gen-ai delete-genai-7` — Delete genai 7
- `digitalocean-pp-cli gen-ai delete-genai-8` — Delete genai 8
- `digitalocean-pp-cli gen-ai delete-genai-9` — Delete genai 9
- `digitalocean-pp-cli gen-ai get` — Get
- `digitalocean-pp-cli gen-ai get-genai` — Get genai
- `digitalocean-pp-cli gen-ai get-genai-10` — Get genai 10
- `digitalocean-pp-cli gen-ai get-genai-11` — Get genai 11
- `digitalocean-pp-cli gen-ai get-genai-12` — Get genai 12
- `digitalocean-pp-cli gen-ai get-genai-13` — Get genai 13
- `digitalocean-pp-cli gen-ai get-genai-14` — Get genai 14
- `digitalocean-pp-cli gen-ai get-genai-15` — Get genai 15
- `digitalocean-pp-cli gen-ai get-genai-16` — Get genai 16
- `digitalocean-pp-cli gen-ai get-genai-17` — Get genai 17
- `digitalocean-pp-cli gen-ai get-genai-18` — Get genai 18
- `digitalocean-pp-cli gen-ai get-genai-19` — Get genai 19
- `digitalocean-pp-cli gen-ai get-genai-2` — Get genai 2
- `digitalocean-pp-cli gen-ai get-genai-20` — Get genai 20
- `digitalocean-pp-cli gen-ai get-genai-21` — Get genai 21
- `digitalocean-pp-cli gen-ai get-genai-22` — Get genai 22
- `digitalocean-pp-cli gen-ai get-genai-23` — Get genai 23
- `digitalocean-pp-cli gen-ai get-genai-24` — Get genai 24
- `digitalocean-pp-cli gen-ai get-genai-25` — Get genai 25
- `digitalocean-pp-cli gen-ai get-genai-26` — Get genai 26
- `digitalocean-pp-cli gen-ai get-genai-3` — Get genai 3
- `digitalocean-pp-cli gen-ai get-genai-4` — Get genai 4
- `digitalocean-pp-cli gen-ai get-genai-5` — Get genai 5
- `digitalocean-pp-cli gen-ai get-genai-6` — Get genai 6
- `digitalocean-pp-cli gen-ai get-genai-7` — Get genai 7
- `digitalocean-pp-cli gen-ai get-genai-8` — Get genai 8
- `digitalocean-pp-cli gen-ai get-genai-9` — Get genai 9
- `digitalocean-pp-cli gen-ai list` — List
- `digitalocean-pp-cli gen-ai list-genai` — List genai
- `digitalocean-pp-cli gen-ai list-genai-10` — List genai 10
- `digitalocean-pp-cli gen-ai list-genai-11` — List genai 11
- `digitalocean-pp-cli gen-ai list-genai-12` — List genai 12
- `digitalocean-pp-cli gen-ai list-genai-13` — List genai 13
- `digitalocean-pp-cli gen-ai list-genai-14` — List genai 14
- `digitalocean-pp-cli gen-ai list-genai-15` — List genai 15
- `digitalocean-pp-cli gen-ai list-genai-16` — List genai 16
- `digitalocean-pp-cli gen-ai list-genai-17` — List genai 17
- `digitalocean-pp-cli gen-ai list-genai-2` — List genai 2
- `digitalocean-pp-cli gen-ai list-genai-3` — List genai 3
- `digitalocean-pp-cli gen-ai list-genai-4` — List genai 4
- `digitalocean-pp-cli gen-ai list-genai-5` — List genai 5
- `digitalocean-pp-cli gen-ai list-genai-6` — List genai 6
- `digitalocean-pp-cli gen-ai list-genai-7` — List genai 7
- `digitalocean-pp-cli gen-ai list-genai-8` — List genai 8
- `digitalocean-pp-cli gen-ai list-genai-9` — List genai 9
- `digitalocean-pp-cli gen-ai update` — Update
- `digitalocean-pp-cli gen-ai update-genai` — Update genai
- `digitalocean-pp-cli gen-ai update-genai-10` — Update genai 10
- `digitalocean-pp-cli gen-ai update-genai-11` — Update genai 11
- `digitalocean-pp-cli gen-ai update-genai-12` — Update genai 12
- `digitalocean-pp-cli gen-ai update-genai-13` — Update genai 13
- `digitalocean-pp-cli gen-ai update-genai-14` — Update genai 14
- `digitalocean-pp-cli gen-ai update-genai-15` — Update genai 15
- `digitalocean-pp-cli gen-ai update-genai-2` — Update genai 2
- `digitalocean-pp-cli gen-ai update-genai-3` — Update genai 3
- `digitalocean-pp-cli gen-ai update-genai-4` — Update genai 4
- `digitalocean-pp-cli gen-ai update-genai-5` — Update genai 5
- `digitalocean-pp-cli gen-ai update-genai-6` — Update genai 6
- `digitalocean-pp-cli gen-ai update-genai-7` — Update genai 7
- `digitalocean-pp-cli gen-ai update-genai-8` — Update genai 8
- `digitalocean-pp-cli gen-ai update-genai-9` — Update genai 9

**images** — A DigitalOcean [image](https://docs.digitalocean.com/products/images/) can be
used to create a Droplet and may come in a number of flavors. Currently,
there are five types of images: snapshots, backups, applications,
distributions, and custom images.

* [Snapshots](https://docs.digitalocean.com/products/snapshots/) provide
a full copy of an existing Droplet instance taken on demand.

* [Backups](https://docs.digitalocean.com/products/backups/) are similar
to snapshots but are created automatically at regular intervals when
enabled for a Droplet.

* [Custom images](https://docs.digitalocean.com/products/custom-images/)
are Linux-based virtual machine images (raw, qcow2, vhdx, vdi, and vmdk
formats are supported) that you may upload for use on DigitalOcean.

* Distributions are the public Linux distributions that are available to
be used as a base to create Droplets.

* Applications, or [1-Click Apps](https://docs.digitalocean.com/products/marketplace/),
are distributions pre-configured with additional software.

To interact with images, you will generally send requests to the images
endpoint at /v2/images.

- `digitalocean-pp-cli images create` — Create
- `digitalocean-pp-cli images create-generations` — Create generations
- `digitalocean-pp-cli images delete` — Delete
- `digitalocean-pp-cli images get` — Get
- `digitalocean-pp-cli images list` — List
- `digitalocean-pp-cli images update` — Update

**kubernetes** — [DigitalOcean Kubernetes](https://docs.digitalocean.com/products/kubernetes/)
allows you to quickly deploy scalable and secure Kubernetes clusters. By
sending requests to the `/v2/kubernetes/clusters` endpoint, you can list,
create, or delete clusters as well as scale node pools up and down,
recycle individual nodes, and retrieve the kubeconfig file for use with
a cluster.

- `digitalocean-pp-cli kubernetes create` — Create
- `digitalocean-pp-cli kubernetes create-clusters` — Create clusters
- `digitalocean-pp-cli kubernetes create-clusters-2` — Create clusters 2
- `digitalocean-pp-cli kubernetes create-clusters-3` — Create clusters 3
- `digitalocean-pp-cli kubernetes create-clusters-4` — Create clusters 4
- `digitalocean-pp-cli kubernetes create-registries` — Create registries
- `digitalocean-pp-cli kubernetes create-registry` — Create registry
- `digitalocean-pp-cli kubernetes delete` — Delete
- `digitalocean-pp-cli kubernetes delete-clusters` — Delete clusters
- `digitalocean-pp-cli kubernetes delete-clusters-2` — Delete clusters 2
- `digitalocean-pp-cli kubernetes delete-clusters-3` — Delete clusters 3
- `digitalocean-pp-cli kubernetes delete-clusters-4` — Delete clusters 4
- `digitalocean-pp-cli kubernetes delete-clusters-5` — Delete clusters 5
- `digitalocean-pp-cli kubernetes delete-registry` — Delete registry
- `digitalocean-pp-cli kubernetes get` — Get
- `digitalocean-pp-cli kubernetes get-clusters` — Get clusters
- `digitalocean-pp-cli kubernetes get-clusters-2` — Get clusters 2
- `digitalocean-pp-cli kubernetes get-clusters-3` — Get clusters 3
- `digitalocean-pp-cli kubernetes get-clusters-4` — Get clusters 4
- `digitalocean-pp-cli kubernetes get-clusters-5` — Get clusters 5
- `digitalocean-pp-cli kubernetes get-clusters-6` — Get clusters 6
- `digitalocean-pp-cli kubernetes get-clusters-7` — Get clusters 7
- `digitalocean-pp-cli kubernetes get-clusters-8` — Get clusters 8
- `digitalocean-pp-cli kubernetes get-clusters-9` — Get clusters 9
- `digitalocean-pp-cli kubernetes list` — List
- `digitalocean-pp-cli kubernetes list-options` — List options
- `digitalocean-pp-cli kubernetes update` — Update
- `digitalocean-pp-cli kubernetes update-clusters` — Update clusters

**load-balancers** — [DigitalOcean Load Balancers](https://docs.digitalocean.com/products/networking/load-balancers/)
provide a way to distribute traffic across multiple Droplets. By sending
requests to the `/v2/load_balancers` endpoint, you can list, create, or
delete load balancers as well as add or remove Droplets, forwarding rules,
and other configuration details.

- `digitalocean-pp-cli load-balancers create` — Create
- `digitalocean-pp-cli load-balancers delete` — Delete
- `digitalocean-pp-cli load-balancers get` — Get
- `digitalocean-pp-cli load-balancers list` — List
- `digitalocean-pp-cli load-balancers update` — Update

**messages** — Manage messages

- `digitalocean-pp-cli messages` — Create

**models** — Manage models

- `digitalocean-pp-cli models` — List

**monitoring** — The DigitalOcean Monitoring API makes it possible to programmatically retrieve metrics as well as configure alert
policies based on these metrics. The Monitoring API can help you gain insight into how your apps are performing
and consuming resources.

- `digitalocean-pp-cli monitoring create` — Create
- `digitalocean-pp-cli monitoring create-sinks` — Create sinks
- `digitalocean-pp-cli monitoring create-sinks-2` — Create sinks 2
- `digitalocean-pp-cli monitoring create-sinks-3` — Create sinks 3
- `digitalocean-pp-cli monitoring delete` — Delete
- `digitalocean-pp-cli monitoring delete-sinks` — Delete sinks
- `digitalocean-pp-cli monitoring delete-sinks-2` — Delete sinks 2
- `digitalocean-pp-cli monitoring get` — Get
- `digitalocean-pp-cli monitoring get-sinks` — Get sinks
- `digitalocean-pp-cli monitoring get-sinks-2` — Get sinks 2
- `digitalocean-pp-cli monitoring list` — List
- `digitalocean-pp-cli monitoring list-metrics` — List metrics
- `digitalocean-pp-cli monitoring list-metrics-10` — List metrics 10
- `digitalocean-pp-cli monitoring list-metrics-11` — List metrics 11
- `digitalocean-pp-cli monitoring list-metrics-12` — List metrics 12
- `digitalocean-pp-cli monitoring list-metrics-13` — List metrics 13
- `digitalocean-pp-cli monitoring list-metrics-14` — List metrics 14
- `digitalocean-pp-cli monitoring list-metrics-15` — List metrics 15
- `digitalocean-pp-cli monitoring list-metrics-16` — List metrics 16
- `digitalocean-pp-cli monitoring list-metrics-17` — List metrics 17
- `digitalocean-pp-cli monitoring list-metrics-18` — List metrics 18
- `digitalocean-pp-cli monitoring list-metrics-19` — List metrics 19
- `digitalocean-pp-cli monitoring list-metrics-2` — List metrics 2
- `digitalocean-pp-cli monitoring list-metrics-20` — List metrics 20
- `digitalocean-pp-cli monitoring list-metrics-21` — List metrics 21
- `digitalocean-pp-cli monitoring list-metrics-22` — List metrics 22
- `digitalocean-pp-cli monitoring list-metrics-23` — List metrics 23
- `digitalocean-pp-cli monitoring list-metrics-24` — List metrics 24
- `digitalocean-pp-cli monitoring list-metrics-25` — List metrics 25
- `digitalocean-pp-cli monitoring list-metrics-26` — List metrics 26
- `digitalocean-pp-cli monitoring list-metrics-27` — List metrics 27
- `digitalocean-pp-cli monitoring list-metrics-28` — List metrics 28
- `digitalocean-pp-cli monitoring list-metrics-29` — List metrics 29
- `digitalocean-pp-cli monitoring list-metrics-3` — List metrics 3
- `digitalocean-pp-cli monitoring list-metrics-30` — List metrics 30
- `digitalocean-pp-cli monitoring list-metrics-31` — List metrics 31
- `digitalocean-pp-cli monitoring list-metrics-32` — List metrics 32
- `digitalocean-pp-cli monitoring list-metrics-33` — List metrics 33
- `digitalocean-pp-cli monitoring list-metrics-34` — List metrics 34
- `digitalocean-pp-cli monitoring list-metrics-35` — List metrics 35
- `digitalocean-pp-cli monitoring list-metrics-36` — List metrics 36
- `digitalocean-pp-cli monitoring list-metrics-37` — List metrics 37
- `digitalocean-pp-cli monitoring list-metrics-38` — List metrics 38
- `digitalocean-pp-cli monitoring list-metrics-39` — List metrics 39
- `digitalocean-pp-cli monitoring list-metrics-4` — List metrics 4
- `digitalocean-pp-cli monitoring list-metrics-40` — List metrics 40
- `digitalocean-pp-cli monitoring list-metrics-41` — List metrics 41
- `digitalocean-pp-cli monitoring list-metrics-42` — List metrics 42
- `digitalocean-pp-cli monitoring list-metrics-43` — List metrics 43
- `digitalocean-pp-cli monitoring list-metrics-44` — List metrics 44
- `digitalocean-pp-cli monitoring list-metrics-45` — List metrics 45
- `digitalocean-pp-cli monitoring list-metrics-46` — List metrics 46
- `digitalocean-pp-cli monitoring list-metrics-47` — List metrics 47
- `digitalocean-pp-cli monitoring list-metrics-48` — List metrics 48
- `digitalocean-pp-cli monitoring list-metrics-49` — List metrics 49
- `digitalocean-pp-cli monitoring list-metrics-5` — List metrics 5
- `digitalocean-pp-cli monitoring list-metrics-50` — List metrics 50
- `digitalocean-pp-cli monitoring list-metrics-51` — List metrics 51
- `digitalocean-pp-cli monitoring list-metrics-52` — List metrics 52
- `digitalocean-pp-cli monitoring list-metrics-53` — List metrics 53
- `digitalocean-pp-cli monitoring list-metrics-54` — List metrics 54
- `digitalocean-pp-cli monitoring list-metrics-55` — List metrics 55
- `digitalocean-pp-cli monitoring list-metrics-56` — List metrics 56
- `digitalocean-pp-cli monitoring list-metrics-57` — List metrics 57
- `digitalocean-pp-cli monitoring list-metrics-58` — List metrics 58
- `digitalocean-pp-cli monitoring list-metrics-6` — List metrics 6
- `digitalocean-pp-cli monitoring list-metrics-7` — List metrics 7
- `digitalocean-pp-cli monitoring list-metrics-8` — List metrics 8
- `digitalocean-pp-cli monitoring list-metrics-9` — List metrics 9
- `digitalocean-pp-cli monitoring list-sinks` — List sinks
- `digitalocean-pp-cli monitoring list-sinks-2` — List sinks 2
- `digitalocean-pp-cli monitoring update` — Update

**nfs** — NFS lets you create fully managed, POSIX-compliant network file storage that delivers secure,
high-performance shared storage right inside your VPC. This enables seamless data sharing across Droplets in a VPC.

- `digitalocean-pp-cli nfs create` — Create
- `digitalocean-pp-cli nfs delete` — Delete
- `digitalocean-pp-cli nfs delete-snapshots` — Delete snapshots
- `digitalocean-pp-cli nfs get` — Get
- `digitalocean-pp-cli nfs get-snapshots` — Get snapshots
- `digitalocean-pp-cli nfs list` — List
- `digitalocean-pp-cli nfs list-snapshots` — List snapshots

**partner-network-connect** — Partner Network Connect lets you establish high-bandwidth, low-latency
network connections directly between DigitalOcean VPC networks and other
public cloud providers or on-premises datacenters.

- `digitalocean-pp-cli partner-network-connect create` — Create
- `digitalocean-pp-cli partner-network-connect create-partnernetworkconnect` — Create partnernetworkconnect
- `digitalocean-pp-cli partner-network-connect delete` — Delete
- `digitalocean-pp-cli partner-network-connect get` — Get
- `digitalocean-pp-cli partner-network-connect get-partnernetworkconnect` — Get partnernetworkconnect
- `digitalocean-pp-cli partner-network-connect get-partnernetworkconnect-2` — Get partnernetworkconnect 2
- `digitalocean-pp-cli partner-network-connect get-partnernetworkconnect-3` — Get partnernetworkconnect 3
- `digitalocean-pp-cli partner-network-connect list` — List
- `digitalocean-pp-cli partner-network-connect update` — Update

**projects** — Projects allow you to organize your resources into groups that fit the way
you work. You can group resources (like Droplets, Spaces, load balancers,
domains, and floating IPs) in ways that align with the applications
you host on DigitalOcean.

- `digitalocean-pp-cli projects create` — Create
- `digitalocean-pp-cli projects create-default` — Create default
- `digitalocean-pp-cli projects delete` — Delete
- `digitalocean-pp-cli projects get` — Get
- `digitalocean-pp-cli projects list` — List
- `digitalocean-pp-cli projects list-default` — List default
- `digitalocean-pp-cli projects list-default-2` — List default 2
- `digitalocean-pp-cli projects update` — Update
- `digitalocean-pp-cli projects update-default` — Update default
- `digitalocean-pp-cli projects update-projectid` — Update projectid
- `digitalocean-pp-cli projects update-projectid-2` — Update projectid 2

**regions** — Provides information about DigitalOcean data center regions.

- `digitalocean-pp-cli regions` — List

**registries** — Manage registries

- `digitalocean-pp-cli registries create` — Create
- `digitalocean-pp-cli registries create-subscription` — Create subscription
- `digitalocean-pp-cli registries create-validatename` — Create validatename
- `digitalocean-pp-cli registries delete` — Delete
- `digitalocean-pp-cli registries get` — Get
- `digitalocean-pp-cli registries list` — List
- `digitalocean-pp-cli registries list-options` — List options
- `digitalocean-pp-cli registries list-subscription` — List subscription

**registry** — Manage registry

- `digitalocean-pp-cli registry create` — Create
- `digitalocean-pp-cli registry create-subscription` — Create subscription
- `digitalocean-pp-cli registry create-validatename` — Create validatename
- `digitalocean-pp-cli registry delete` — Delete
- `digitalocean-pp-cli registry list` — List
- `digitalocean-pp-cli registry list-dockercredentials` — List dockercredentials
- `digitalocean-pp-cli registry list-options` — List options
- `digitalocean-pp-cli registry list-subscription` — List subscription

**reports** — Manage reports

- `digitalocean-pp-cli reports` — List

**reserved-ips** — As of 16 June 2022, we have renamed the [Floating IP](https://docs.digitalocean.com/reference/api/api-reference/#tag/Floating-IPs)
product to Reserved IPs. The Reserved IP product's endpoints function the exact
same way as Floating IPs. The only difference is the name change throughout the
URLs and fields. For example, the `floating_ips` field is now the `reserved_ips` field.
The Floating IP endpoints will remain active until fall 2023 before being
permanently deprecated.

With the exception of the [Projects API](https://docs.digitalocean.com/reference/api/api-reference/#tag/Projects),
we will reflect this change as an additional field in the responses across the API
where the `floating_ip` field is used. For example, the Droplet metadata response
will contain the field `reserved_ips` in addition to the `floating_ips` field.
Floating IPs retrieved using the Projects API will retain the original name.

DigitalOcean Reserved IPs are publicly-accessible static IP addresses that can be
mapped to one of your Droplets. They can be used to create highly available
setups or other configurations requiring movable addresses.

Reserved IPs are bound to a specific region.

- `digitalocean-pp-cli reserved-ips create` — Create
- `digitalocean-pp-cli reserved-ips delete` — Delete
- `digitalocean-pp-cli reserved-ips get` — Get
- `digitalocean-pp-cli reserved-ips list` — List

**reserved-ipv6** — DigitalOcean Reserved IPv6s are publicly-accessible static IP addresses that can be
mapped to one of your Droplets. They can be used to create highly available
setups or other configurations requiring movable addresses.

Reserved IPv6s are bound to a specific region.

- `digitalocean-pp-cli reserved-ipv6 create` — Create
- `digitalocean-pp-cli reserved-ipv6 delete` — Delete
- `digitalocean-pp-cli reserved-ipv6 get` — Get
- `digitalocean-pp-cli reserved-ipv6 list` — List

**responses** — Manage responses

- `digitalocean-pp-cli responses` — Create

**security** — Security CSPM endpoints for scans, scan findings, and settings.

- `digitalocean-pp-cli security create` — Create
- `digitalocean-pp-cli security create-scans` — Create scans
- `digitalocean-pp-cli security create-settings` — Create settings
- `digitalocean-pp-cli security delete` — Delete
- `digitalocean-pp-cli security get` — Get
- `digitalocean-pp-cli security get-scans` — Get scans
- `digitalocean-pp-cli security list` — List
- `digitalocean-pp-cli security list-scans` — List scans
- `digitalocean-pp-cli security list-settings` — List settings
- `digitalocean-pp-cli security update` — Update

**sizes** — The sizes objects represent different packages of hardware resources that
can be used for Droplets. When a Droplet is created, a size must be
selected so that the correct resources can be allocated.

Each size represents a plan that bundles together specific sets of
resources. This includes the amount of RAM, the number of virtual CPUs,
disk space, and transfer. The size object also includes the pricing
details and the regions that the size is available in.

- `digitalocean-pp-cli sizes` — List

**snapshots** — [Snapshots](https://docs.digitalocean.com/products/snapshots/) are saved
instances of a Droplet or a block storage volume, which is reflected in
the `resource_type` attribute. In order to avoid problems with compressing
filesystems, each defines a `min_disk_size` attribute which is the minimum
size of the Droplet or volume disk when creating a new resource from the
saved snapshot.

To interact with snapshots, you will generally send requests to the
snapshots endpoint at `/v2/snapshots`.

- `digitalocean-pp-cli snapshots delete` — Delete
- `digitalocean-pp-cli snapshots get` — Get
- `digitalocean-pp-cli snapshots list` — List

**spaces** — Manage spaces

- `digitalocean-pp-cli spaces create` — Create
- `digitalocean-pp-cli spaces delete` — Delete
- `digitalocean-pp-cli spaces get` — Get
- `digitalocean-pp-cli spaces list` — List
- `digitalocean-pp-cli spaces update` — Update
- `digitalocean-pp-cli spaces update-keys` — Update keys

**tags** — A tag is a label that can be applied to a resource (currently Droplets,
Images, Volumes, Volume Snapshots, and Database clusters) in order to
better organize or facilitate the lookups and actions on it.

Tags have two attributes: a user defined `name` attribute and an embedded
`resources` attribute with information about resources that have been tagged.

- `digitalocean-pp-cli tags create` — Create
- `digitalocean-pp-cli tags delete` — Delete
- `digitalocean-pp-cli tags get` — Get
- `digitalocean-pp-cli tags list` — List

**upload-url** — Manage upload url

- `digitalocean-pp-cli upload-url` — Update

**uptime** — [DigitalOcean Uptime Checks](https://docs.digitalocean.com/products/uptime/) provide the ability to monitor your endpoints from around the world, and alert you when they're slow, unavailable, or SSL certificates are expiring.
To interact with Uptime, you will generally send requests to the Uptime endpoint at `/v2/uptime/`.

- `digitalocean-pp-cli uptime create` — Create
- `digitalocean-pp-cli uptime create-checks` — Create checks
- `digitalocean-pp-cli uptime delete` — Delete
- `digitalocean-pp-cli uptime delete-checks` — Delete checks
- `digitalocean-pp-cli uptime get` — Get
- `digitalocean-pp-cli uptime get-checks` — Get checks
- `digitalocean-pp-cli uptime get-checks-2` — Get checks 2
- `digitalocean-pp-cli uptime get-checks-3` — Get checks 3
- `digitalocean-pp-cli uptime list` — List
- `digitalocean-pp-cli uptime update` — Update
- `digitalocean-pp-cli uptime update-checks` — Update checks

**volumes** — Manage volumes

- `digitalocean-pp-cli volumes create` — Create
- `digitalocean-pp-cli volumes create-actions` — Create actions
- `digitalocean-pp-cli volumes delete` — Delete
- `digitalocean-pp-cli volumes delete-snapshots` — Delete snapshots
- `digitalocean-pp-cli volumes delete-volumeid` — Delete volumeid
- `digitalocean-pp-cli volumes get` — Get
- `digitalocean-pp-cli volumes get-snapshots` — Get snapshots
- `digitalocean-pp-cli volumes list` — List

**vpc-nat-gateways** — [VPC NAT Gateways](https://docs.digitalocean.com/products/networking/vpc/how-to/create-nat-gateway/)
allow resources in a private VPC to access the public internet without
exposing them to incoming traffic.

By sending requests to the `/v2/vpc_nat_gateways` endpoint, you can create,
configure, list, and delete VPC NAT Gateways as well as retrieve information
about the resources assigned to them.

- `digitalocean-pp-cli vpc-nat-gateways create` — Create
- `digitalocean-pp-cli vpc-nat-gateways delete` — Delete
- `digitalocean-pp-cli vpc-nat-gateways get` — Get
- `digitalocean-pp-cli vpc-nat-gateways list` — List
- `digitalocean-pp-cli vpc-nat-gateways update` — Update

**vpc-peerings** — [VPC Peerings](https://docs.digitalocean.com/products/networking/vpc/how-to/create-peering/)
join two VPC networks with a secure, private connection. This allows
resources in those networks to connect to each other's private IP addresses
as if they were in the same network.

- `digitalocean-pp-cli vpc-peerings create` — Create
- `digitalocean-pp-cli vpc-peerings delete` — Delete
- `digitalocean-pp-cli vpc-peerings get` — Get
- `digitalocean-pp-cli vpc-peerings list` — List
- `digitalocean-pp-cli vpc-peerings update` — Update

**vpcs** — [VPCs (virtual private clouds)](https://docs.digitalocean.com/products/networking/vpc/)
allow you to create virtual networks containing resources that can
communicate with each other in full isolation using private IP addresses.

By sending requests to the `/v2/vpcs` endpoint, you can create, configure,
list, and delete custom VPCs as well as retrieve information about the
resources assigned to them.

- `digitalocean-pp-cli vpcs create` — Create
- `digitalocean-pp-cli vpcs delete` — Delete
- `digitalocean-pp-cli vpcs get` — Get
- `digitalocean-pp-cli vpcs list` — List
- `digitalocean-pp-cli vpcs update` — Update
- `digitalocean-pp-cli vpcs update-vpcid` — Update vpcid


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
digitalocean-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup

Store your access token:

```bash
digitalocean-pp-cli auth set-token YOUR_TOKEN_HERE
```

Or set `DIGITALOCEAN_BEARER_AUTH` as an environment variable.

Run `digitalocean-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  digitalocean-pp-cli 1-clicks list --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success, and `--ignore-missing` only when a missing delete target should count as success

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal — piped/agent consumers get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
digitalocean-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
digitalocean-pp-cli feedback --stdin < notes.txt
digitalocean-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.digitalocean-pp-cli/feedback.jsonl`. They are never POSTed unless `DIGITALOCEAN_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `DIGITALOCEAN_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

Write what *surprised* you, not a bug report. Short, specific, one line: that is the part that compounds.

## Output Delivery

Every command accepts `--deliver <sink>`. The output goes to the named sink in addition to (or instead of) stdout, so agents can route command results without hand-piping. Three sinks are supported:

| Sink | Effect |
|------|--------|
| `stdout` | Default; write to stdout only |
| `file:<path>` | Atomically write output to `<path>` (tmp + rename) |
| `webhook:<url>` | POST the output body to the URL (`application/json` or `application/x-ndjson` when `--compact`) |

Unknown schemes are refused with a structured error naming the supported set. Webhook failures return non-zero and log the URL + HTTP status on stderr.

## Named Profiles

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled agent calls the same command every run with the same configuration - HeyGen's "Beacon" pattern.

```
digitalocean-pp-cli profile save briefing --json
digitalocean-pp-cli --profile briefing 1-clicks list
digitalocean-pp-cli profile list --json
digitalocean-pp-cli profile show briefing
digitalocean-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 4 | Authentication required |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `digitalocean-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/cloud/digitalocean/cmd/digitalocean-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add digitalocean-pp-mcp -- digitalocean-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which digitalocean-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   digitalocean-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `digitalocean-pp-cli <command> --help`.
