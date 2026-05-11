# Jira Cloud Platform CLI

Jira Cloud platform REST API documentation

Learn more at [Jira Cloud Platform](http://www.atlassian.com).

## Install

The recommended path installs both the `jira-cloud-platform-pp-cli` binary and the `pp-jira-cloud-platform` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install jira-cloud-platform
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install jira-cloud-platform --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/jira-cloud-platform-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-jira-cloud-platform --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-jira-cloud-platform --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-jira-cloud-platform skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-jira-cloud-platform. The skill defines how its required CLI can be installed.
```

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Set Up Credentials

Get your access token from your API provider's developer portal, then store it:

```bash
jira-cloud-platform-pp-cli auth set-token YOUR_TOKEN_HERE
```

Or set it via environment variable:

```bash
export JIRA_CLOUD_PLATFORM_OAUTH2="your-token-here"
```

### 3. Verify Setup

```bash
jira-cloud-platform-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
jira-cloud-platform-pp-cli attachment get mock-value
```

## Usage

Run `jira-cloud-platform-pp-cli --help` for the full command reference and flag list.

## Commands

### announcement-banner

This resource represents an announcement banner. Use it to retrieve and update banner configuration.

- **`jira-cloud-platform-pp-cli announcement-banner get-banner`** - Returns the current announcement banner configuration.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli announcement-banner set-banner`** - Updates the announcement banner configuration.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).

### app

Manage app

- **`jira-cloud-platform-pp-cli app get-custom-field-configuration`** - Returns a [paginated](#pagination) list of configurations for a custom field of a [type](https://developer.atlassian.com/platform/forge/manifest-reference/modules/jira-custom-field-type/) created by a [Forge app](https://developer.atlassian.com/platform/forge/).

The result can be filtered by one of these criteria:

 *  `id`.
 *  `fieldContextId`.
 *  `issueId`.
 *  `projectKeyOrId` and `issueTypeId`.

Otherwise, all configurations are returned.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg). Jira permissions are not required for the Forge app that provided the custom field type.
- **`jira-cloud-platform-pp-cli app get-custom-fields-configurations`** - Returns a [paginated](#pagination) list of configurations for list of custom fields of a [type](https://developer.atlassian.com/platform/forge/manifest-reference/modules/jira-custom-field-type/) created by a [Forge app](https://developer.atlassian.com/platform/forge/).

The result can be filtered by one of these criteria:

 *  `id`.
 *  `fieldContextId`.
 *  `issueId`.
 *  `projectKeyOrId` and `issueTypeId`.

Otherwise, all configurations for the provided list of custom fields are returned.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg). Jira permissions are not required for the Forge app that provided the custom field type.
- **`jira-cloud-platform-pp-cli app update-custom-field-configuration`** - Update the configuration for contexts of a custom field of a [type](https://developer.atlassian.com/platform/forge/manifest-reference/modules/jira-custom-field-type/) created by a [Forge app](https://developer.atlassian.com/platform/forge/).

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg). Jira permissions are not required for the Forge app that created the custom field type.
- **`jira-cloud-platform-pp-cli app update-custom-field-value`** - Updates the value of a custom field on one or more issues.

Apps can only perform this operation on [custom fields](https://developer.atlassian.com/platform/forge/manifest-reference/modules/jira-custom-field/) and [custom field types](https://developer.atlassian.com/platform/forge/manifest-reference/modules/jira-custom-field-type/) declared in their own manifests.

**[Permissions](#permissions) required:** Only the app that owns the custom field or custom field type can update its values with this operation.

The new `write:app-data:jira` OAuth scope is 100% optional now, and not using it won't break your app. However, we recommend adding it to your app's scope list because we will eventually make it mandatory.
- **`jira-cloud-platform-pp-cli app update-multiple-custom-field-values`** - Updates the value of one or more custom fields on one or more issues. Combinations of custom field and issue should be unique within the request.

Apps can only perform this operation on [custom fields](https://developer.atlassian.com/platform/forge/manifest-reference/modules/jira-custom-field/) and [custom field types](https://developer.atlassian.com/platform/forge/manifest-reference/modules/jira-custom-field-type/) declared in their own manifests.

**[Permissions](#permissions) required:** Only the app that owns the custom field or custom field type can update its values with this operation.

The new `write:app-data:jira` OAuth scope is 100% optional now, and not using it won't break your app. However, we recommend adding it to your app's scope list because we will eventually make it mandatory.

### application-properties

Manage application properties

- **`jira-cloud-platform-pp-cli application-properties get-advanced-settings`** - Returns the application properties that are accessible on the *Advanced Settings* page. To navigate to the *Advanced Settings* page in Jira, choose the Jira icon > **Jira settings** > **System**, **General Configuration** and then click **Advanced Settings** (in the upper right).

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli application-properties get-application-property`** - Returns all application properties or an application property.

If you specify a value for the `key` parameter, then an application property is returned as an object (not in an array). Otherwise, an array of all editable application properties is returned. See [Set application property](#api-rest-api-3-application-properties-id-put) for descriptions of editable properties.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli application-properties set-application-property`** - Changes the value of an application property. For example, you can change the value of the `jira.clone.prefix` from its default value of *CLONE -* to *Clone -* if you prefer sentence case capitalization. Editable properties are described below along with their default values.

#### Advanced settings ####

The advanced settings below are also accessible in [Jira](https://confluence.atlassian.com/x/vYXKM).

| Key | Description | Default value |  
| -- | -- | -- |  
| `jira.clone.prefix` | The string of text prefixed to the title of a cloned issue. | `CLONE -` |  
| `jira.date.picker.java.format` | The date format for the Java (server-side) generated dates. This must be the same as the `jira.date.picker.javascript.format` format setting. | `d/MMM/yy` |  
| `jira.date.picker.javascript.format` | The date format for the JavaScript (client-side) generated dates. This must be the same as the `jira.date.picker.java.format` format setting. | `%e/%b/%y` |  
| `jira.date.time.picker.java.format` | The date format for the Java (server-side) generated date times. This must be the same as the `jira.date.time.picker.javascript.format` format setting. | `dd/MMM/yy h:mm a` |  
| `jira.date.time.picker.javascript.format` | The date format for the JavaScript (client-side) generated date times. This must be the same as the `jira.date.time.picker.java.format` format setting. | `%e/%b/%y %I:%M %p` |  
| `jira.issue.actions.order` | The default order of actions (such as *Comments* or *Change history*) displayed on the issue view. | `asc` |  
| `jira.view.issue.links.sort.order` | The sort order of the list of issue links on the issue view. | `type, status, priority` |  
| `jira.comment.collapsing.minimum.hidden` | The minimum number of comments required for comment collapsing to occur. A value of `0` disables comment collapsing. | `4` |  
| `jira.newsletter.tip.delay.days` | The number of days before a prompt to sign up to the Jira Insiders newsletter is shown. A value of `-1` disables this feature. | `7` |  


#### Look and feel ####

The settings listed below adjust the [look and feel](https://confluence.atlassian.com/x/VwCLLg).

| Key | Description | Default value |  
| -- | -- | -- |  
| `jira.lf.date.time` | The [ time format](https://docs.oracle.com/javase/6/docs/api/index.html?java/text/SimpleDateFormat.html). | `h:mm a` |  
| `jira.lf.date.day` | The [ day format](https://docs.oracle.com/javase/6/docs/api/index.html?java/text/SimpleDateFormat.html). | `EEEE h:mm a` |  
| `jira.lf.date.complete` | The [ date and time format](https://docs.oracle.com/javase/6/docs/api/index.html?java/text/SimpleDateFormat.html). | `dd/MMM/yy h:mm a` |  
| `jira.lf.date.dmy` | The [ date format](https://docs.oracle.com/javase/6/docs/api/index.html?java/text/SimpleDateFormat.html). | `dd/MMM/yy` |  
| `jira.date.time.picker.use.iso8061` | When enabled, sets Monday as the first day of the week in the date picker, as specified by the ISO8601 standard. | `false` |  
| `jira.lf.logo.url` | The URL of the logo image file. | `/images/icon-jira-logo.png` |  
| `jira.lf.logo.show.application.title` | Controls the visibility of the application title on the sidebar. | `false` |  
| `jira.lf.favicon.url` | The URL of the favicon. | `/favicon.ico` |  
| `jira.lf.favicon.hires.url` | The URL of the high-resolution favicon. | `/images/64jira.png` |  
| `jira.lf.navigation.bgcolour` | The background color of the sidebar. | `#0747A6` |  
| `jira.lf.navigation.highlightcolour` | The color of the text and logo of the sidebar. | `#DEEBFF` |  
| `jira.lf.hero.button.base.bg.colour` | The background color of the hero button. | `#3b7fc4` |  
| `jira.title` | The text for the application title. The application title can also be set in *General settings*. | `Jira` |  
| `jira.option.globalsharing` | Whether filters and dashboards can be shared with anyone signed into Jira. | `true` |  
| `xflow.product.suggestions.enabled` | Whether to expose product suggestions for other Atlassian products within Jira. | `true` |  


#### Other settings ####

| Key | Description | Default value |  
| -- | -- | -- |  
| `jira.issuenav.criteria.autoupdate` | Whether instant updates to search criteria is active. | `true` |  


*Note: Be careful when changing [application properties and advanced settings](https://confluence.atlassian.com/x/vYXKM).*

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).

### applicationrole

Manage applicationrole

- **`jira-cloud-platform-pp-cli applicationrole get-all-application-roles`** - Returns all application roles. In Jira, application roles are managed using the [Application access configuration](https://confluence.atlassian.com/x/3YxjL) page.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli applicationrole get-application-role`** - Returns an application role.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).

### atlassian-connect

Manage atlassian connect

- **`jira-cloud-platform-pp-cli atlassian-connect addon-properties-resource-delete-addon-property-delete`** - Deletes an app's property.

**[Permissions](#permissions) required:** Only a Connect app whose key matches `addonKey` can make this request.
Additionally, Forge apps can access Connect app properties (stored against the same `app.connect.key`).
- **`jira-cloud-platform-pp-cli atlassian-connect addon-properties-resource-get-addon-properties-get`** - Gets all the properties of an app. The reserved key `connect_client_key_019cdff3-8bfb-71fe-9628-875b700aebb8` is not returned.

**[Permissions](#permissions) required:** Only a Connect app whose key matches `addonKey` can make this request.
Additionally, Forge apps can access Connect app properties (stored against the same `app.connect.key`).
- **`jira-cloud-platform-pp-cli atlassian-connect addon-properties-resource-get-addon-property-get`** - Returns the key and value of an app's property. The property key `connect_client_key_019cdff3-8bfb-71fe-9628-875b700aebb8`
is reserved. It returns a synthetic, read-only property containing the Connect `clientKey` for the requested tenant.
This is intended for Forge apps with `app.connect.key` to retrieve the Connect client key during migration.

**[Permissions](#permissions) required:** Only a Connect app whose key matches `addonKey` can make this request.
Additionally, Forge apps can access Connect app properties (stored against the same `app.connect.key`).
- **`jira-cloud-platform-pp-cli atlassian-connect addon-properties-resource-put-addon-property-put`** - Sets the value of an app's property. Use this resource to store custom data for your app.

The value of the request body must be a [valid](http://tools.ietf.org/html/rfc4627), non-empty JSON blob. The maximum length is 32768 characters.

**[Permissions](#permissions) required:** Only a Connect app whose key matches `addonKey` can make this request.
Additionally, Forge apps can access Connect app properties (stored against the same `app.connect.key`).
- **`jira-cloud-platform-pp-cli atlassian-connect app-issue-field-value-update-resource-update-issue-fields-put`** - Updates the value of a custom field added by Connect apps on one or more issues.
The values of up to 200 custom fields can be updated.

**[Permissions](#permissions) required:** Only Connect apps can make this request
- **`jira-cloud-platform-pp-cli atlassian-connect connect-to-forge-migration-fetch-task-resource-fetch-migration-task-get`** - Returns the details of a Connect issue field's migration to Forge.

When migrating a Connect app to Forge, [Issue Field](https://developer.atlassian.com/cloud/jira/software/modules/issue-field/) modules
must be converted to [Custom field](https://developer.atlassian.com/platform/forge/manifest-reference/modules/jira-custom-field/). When the
Forge version of the app is installed, Forge creates a
[background task](https://developer.atlassian.com/cloud/jira/platform/rest/v3/api-group-tasks/#api-group-tasks) to track the
migration of field data across. This endpoint returns the status and other details of that background task.

For more details, see
[Jira modules > Jira Custom Fields](https://developer.atlassian.com/platform/adopting-forge-from-connect/migrate-jira-custom-fields/).

**[Permissions](#permissions) required:** Only Connect and Forge apps can make this request.
- **`jira-cloud-platform-pp-cli atlassian-connect connect-to-forge-migration-task-submission-resource-submit-task-post`** - Submits a request to trigger migration of connect issue field to its Forge custom field counterpart.

When migrating a Connect app to Forge, [Issue Field](https://developer.atlassian.com/cloud/jira/software/modules/issue-field/) modules
must be converted to [Custom field](https://developer.atlassian.com/platform/forge/manifest-reference/modules/jira-custom-field/) modules.
This endpoint triggers the background migration of field data. Use the GET endpoint to retrieve
the status and progress of the task.

For more details, see
[Jira modules > Jira Custom Fields](https://developer.atlassian.com/platform/adopting-forge-from-connect/migrate-jira-custom-fields/).

**[Permissions](#permissions) required:** Only Connect and Forge apps can make this request.
- **`jira-cloud-platform-pp-cli atlassian-connect dynamic-modules-resource-get-modules-get`** - Returns all modules registered dynamically by the calling app.

**[Permissions](#permissions) required:** Only Connect apps can make this request.
- **`jira-cloud-platform-pp-cli atlassian-connect dynamic-modules-resource-register-modules-post`** - Registers a list of modules.

**[Permissions](#permissions) required:** Only Connect apps can make this request.
- **`jira-cloud-platform-pp-cli atlassian-connect dynamic-modules-resource-remove-modules-delete`** - Remove all or a list of modules registered by the calling app.

**[Permissions](#permissions) required:** Only Connect apps can make this request.
- **`jira-cloud-platform-pp-cli atlassian-connect migration-resource-update-entity-properties-value-put`** - Updates the values of multiple entity properties for an object, up to 50 updates per request. This operation is for use by Connect apps during app migration.
- **`jira-cloud-platform-pp-cli atlassian-connect migration-resource-workflow-rule-search-post`** - Returns configurations for workflow transition rules migrated from server to cloud and owned by the calling Connect app.
- **`jira-cloud-platform-pp-cli atlassian-connect service-registry-resource-services-get`** - Retrieve the attributes of given service registries.

**[Permissions](#permissions) required:** Only Connect apps can make this request and the servicesIds belong to the tenant you are requesting

### attachment

Manage attachment

- **`jira-cloud-platform-pp-cli attachment get`** - Returns the metadata for an attachment. Note that the attachment itself is not returned.

This operation can be accessed anonymously.

**[Permissions](#permissions) required:**

 *  *Browse projects* [project permission](https://confluence.atlassian.com/x/yodKLg) for the project that the issue is in.
 *  If [issue-level security](https://confluence.atlassian.com/x/J4lKLg) is configured, issue-level security permission to view the issue.
 *  If attachments are added in private comments, the comment-level restriction will be applied.
- **`jira-cloud-platform-pp-cli attachment get-content`** - Returns the contents of an attachment. A `Range` header can be set to define a range of bytes within the attachment to download. See the [HTTP Range header standard](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Range) for details.

To return a thumbnail of the attachment, use [Get attachment thumbnail](#api-rest-api-3-attachment-thumbnail-id-get).

This operation can be accessed anonymously.

**[Permissions](#permissions) required:** For the issue containing the attachment:

 *  *Browse projects* [project permission](https://confluence.atlassian.com/x/yodKLg) for the project that the issue is in.
 *  If [issue-level security](https://confluence.atlassian.com/x/J4lKLg) is configured, issue-level security permission to view the issue.
 *  If attachments are added in private comments, the comment-level restriction will be applied.
- **`jira-cloud-platform-pp-cli attachment get-meta`** - Returns the attachment settings, that is, whether attachments are enabled and the maximum attachment size allowed.

Note that there are also [project permissions](https://confluence.atlassian.com/x/yodKLg) that restrict whether users can create and delete attachments.

This operation can be accessed anonymously.

**[Permissions](#permissions) required:** None.
- **`jira-cloud-platform-pp-cli attachment get-thumbnail`** - Returns the thumbnail of an attachment.

To return the attachment contents, use [Get attachment content](#api-rest-api-3-attachment-content-id-get).

This operation can be accessed anonymously.

**[Permissions](#permissions) required:** For the issue containing the attachment:

 *  *Browse projects* [project permission](https://confluence.atlassian.com/x/yodKLg) for the project that the issue is in.
 *  If [issue-level security](https://confluence.atlassian.com/x/J4lKLg) is configured, issue-level security permission to view the issue.
 *  If attachments are added in private comments, the comment-level restriction will be applied.
- **`jira-cloud-platform-pp-cli attachment remove`** - Deletes an attachment from an issue.

This operation can be accessed anonymously.

**[Permissions](#permissions) required:** For the project holding the issue containing the attachment:

 *  *Delete own attachments* [project permission](https://confluence.atlassian.com/x/yodKLg) to delete an attachment created by the calling user.
 *  *Delete all attachments* [project permission](https://confluence.atlassian.com/x/yodKLg) to delete an attachment created by any user.

### auditing

Manage auditing

- **`jira-cloud-platform-pp-cli auditing get-audit-records`** - Returns a list of audit records. The list can be filtered to include items:

 *  where each item in `filter` has at least one match in any of these fields:
    
     *  `summary`
     *  `category`
     *  `eventSource`
     *  `objectItem.name` If the object is a user, account ID is available to filter.
     *  `objectItem.parentName`
     *  `objectItem.typeName`
     *  `changedValues.changedFrom`
     *  `changedValues.changedTo`
     *  `remoteAddress`
    
    For example, if `filter` contains *man ed*, an audit record containing `summary": "User added to group"` and `"category": "group management"` is returned.
 *  created on or after a date and time.
 *  created or or before a date and time.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).

### avatar

This resource represents system and custom avatars. Use it to obtain the details of system or custom avatars, add and remove avatars from a project, issue type or priority and obtain avatar images.


### bulk

Manage bulk

- **`jira-cloud-platform-pp-cli bulk get-available-transitions`** - Use this API to retrieve a list of transitions available for the specified issues that can be used or bulk transition operations. You can submit either single or multiple issues in the query to obtain the available transitions.

The response will provide the available transitions for issues, organized by their respective workflows. **Only the transitions that are common among the issues within that workflow and do not involve any additional field updates will be included.** For bulk transitions that require additional field updates, please utilise the Jira Cloud UI.

You can request available transitions for up to 1,000 issues in a single operation. This API uses pagination to return responses, delivering 50 workflows at a time.

**[Permissions](#permissions) required:**

 *  Global bulk change [permission](https://support.atlassian.com/jira-cloud-administration/docs/manage-global-permissions/).
 *  Transition [issues permission](https://support.atlassian.com/jira-cloud-administration/docs/permissions-for-company-managed-projects/#Transition-issues/) in all projects that contain the selected issues.
 *  Browse [project permission](https://support.atlassian.com/jira-cloud-administration/docs/manage-project-permissions/) in all projects that contain the selected issues.
 *  If [issue-level security](https://confluence.atlassian.com/x/J4lKLg) is configured, issue-level security permission to view the issue.
- **`jira-cloud-platform-pp-cli bulk get-editable-fields`** - Use this API to get a list of fields visible to the user to perform bulk edit operations. You can pass single or multiple issues in the query to get eligible editable fields. This API uses pagination to return responses, delivering 50 fields at a time.

**[Permissions](#permissions) required:**

 *  Global bulk change [permission](https://support.atlassian.com/jira-cloud-administration/docs/manage-global-permissions/).
 *  Browse [project permission](https://support.atlassian.com/jira-cloud-administration/docs/manage-project-permissions/) in all projects that contain the selected issues.
 *  If [issue-level security](https://confluence.atlassian.com/x/J4lKLg) is configured, issue-level security permission to view the issue.
 *  Depending on the field, any field-specific permissions required to edit it.
- **`jira-cloud-platform-pp-cli bulk get-operation-progress`** - Use this to get the progress state for the specified bulk operation `taskId`.

**[Permissions](#permissions) required:**

 *  Global bulk change [permission](https://support.atlassian.com/jira-cloud-administration/docs/manage-global-permissions/).

If the task is running, this resource will return:

    {"taskId":"10779","status":"RUNNING","progressPercent":65,"submittedBy":{"accountId":"5b10a2844c20165700ede21g"},"created":1690180055963,"started":1690180056206,"updated":169018005829}

If the task has completed, then this resource will return:

    {"processedAccessibleIssues":[10001,10002],"created":1709189449954,"progressPercent":100,"started":1709189450154,"status":"COMPLETE","submittedBy":{"accountId":"5b10a2844c20165700ede21g"},"invalidOrInaccessibleIssueCount":0,"taskId":"10000","totalIssueCount":2,"updated":1709189450354}

**Note:** You can view task progress for up to 14 days from creation.
- **`jira-cloud-platform-pp-cli bulk submit-delete`** - Use this API to submit a bulk delete request. You can delete up to 1,000 issues in a single operation.

**[Permissions](#permissions) required:**

 *  Global bulk change [permission](https://support.atlassian.com/jira-cloud-administration/docs/manage-global-permissions/).
 *  Delete [issues permission](https://support.atlassian.com/jira-cloud-administration/docs/permissions-for-company-managed-projects/#Delete-issues/) in all projects that contain the selected issues.
 *  Browse [project permission](https://support.atlassian.com/jira-cloud-administration/docs/manage-project-permissions/) in all projects that contain the selected issues.
 *  If [issue-level security](https://confluence.atlassian.com/x/J4lKLg) is configured, issue-level security permission to view the issue.
- **`jira-cloud-platform-pp-cli bulk submit-edit`** - Use this API to submit a bulk edit request and simultaneously edit multiple issues. There are limits applied to the number of issues and fields that can be edited. A single request can accommodate a maximum of 1000 issues (including subtasks) and 200 fields.

**[Permissions](#permissions) required:**

 *  Global bulk change [permission](https://support.atlassian.com/jira-cloud-administration/docs/manage-global-permissions/).
 *  Browse [project permission](https://support.atlassian.com/jira-cloud-administration/docs/manage-project-permissions/) in all projects that contain the selected issues.
 *  Edit [issues permission](https://support.atlassian.com/jira-cloud-administration/docs/manage-project-permissions/) in all projects that contain the selected issues.
 *  If [issue-level security](https://confluence.atlassian.com/x/J4lKLg) is configured, issue-level security permission to view the issue.
- **`jira-cloud-platform-pp-cli bulk submit-move`** - Use this API to submit a bulk issue move request. You can move multiple issues from multiple projects in a single request, but they must all be moved to a single project, issue type, and parent. You can't move more than 1000 issues (including subtasks) at once.

#### Scenarios: ####

This is an early version of the API and it doesn't have full feature parity with the Bulk Move UI experience.

 *  Moving issue of type A to issue of type B in the same project or a different project: `SUPPORTED`
 *  Moving multiple issues of type A in one or more projects to multiple issues of type B in one of the source projects or a different project: `SUPPORTED`
 *  Moving issues of multiple issue types in one or more projects to issues of a single issue type in one of the source project or a different project: **`SUPPORTED`**  
    E.g. Moving issues of story and task issue types in project 1 and project 2 to issues of task issue type in project 3
 *  Moving a standard parent issue of type A with its multiple subtask issue types in one project to standard issue of type B and multiple subtask issue types in the same project or a different project: `SUPPORTED`
 *  Moving standard issues with their subtasks to a parent issue in the same project or a different project without losing their relation: `SUPPORTED`
 *  Moving an epic issue with its child issues to a different project without losing their relation: `SUPPORTED`  
    This usecase is **supported using multiple requests**. Move the epic in one request and then move the children in a separate request with target parent set to the epic issue id  
      
    (Alternatively, move them individually and stitch the relationship back with the Bulk Edit API)

#### Limits applied to bulk issue moves: ####

When using the bulk move, keep in mind that there are limits on the number of issues and fields you can include.

 *  You can move up to 1,000 issues in a single operation, including any subtasks.
 *  The total combined number of fields across all issues must not exceed 1,500,000. For example, if each issue includes 15,000 fields, then the maximum number of issues that can be moved is 100.

**[Permissions](#permissions) required:**

 *  Global bulk change [permission](https://support.atlassian.com/jira-cloud-administration/docs/manage-global-permissions/).
 *  Move [issues permission](https://support.atlassian.com/jira-cloud-administration/docs/manage-project-permissions/) in source projects.
 *  Create [issues permission](https://support.atlassian.com/jira-cloud-administration/docs/manage-project-permissions/) in destination projects.
 *  Browse [project permission](https://support.atlassian.com/jira-cloud-administration/docs/manage-project-permissions/) in destination projects, if moving subtasks only.
 *  If [issue-level security](https://confluence.atlassian.com/x/J4lKLg) is configured, issue-level security permission to view the issue.
- **`jira-cloud-platform-pp-cli bulk submit-transition`** - Use this API to submit a bulk issue status transition request. You can transition multiple issues, alongside with their valid transition Ids. You can transition up to 1,000 issues in a single operation.

**[Permissions](#permissions) required:**

 *  Global bulk change [permission](https://support.atlassian.com/jira-cloud-administration/docs/manage-global-permissions/).
 *  Transition [issues permission](https://support.atlassian.com/jira-cloud-administration/docs/permissions-for-company-managed-projects/#Transition-issues/) in all projects that contain the selected issues.
 *  Browse [project permission](https://support.atlassian.com/jira-cloud-administration/docs/manage-project-permissions/) in all projects that contain the selected issues.
 *  If [issue-level security](https://confluence.atlassian.com/x/J4lKLg) is configured, issue-level security permission to view the issue.
- **`jira-cloud-platform-pp-cli bulk submit-unwatch`** - Use this API to submit a bulk unwatch request. You can unwatch up to 1,000 issues in a single operation.

**[Permissions](#permissions) required:**

 *  Global bulk change [permission](https://support.atlassian.com/jira-cloud-administration/docs/manage-global-permissions/).
 *  Browse [project permission](https://support.atlassian.com/jira-cloud-administration/docs/manage-project-permissions/) in all projects that contain the selected issues.
 *  If [issue-level security](https://confluence.atlassian.com/x/J4lKLg) is configured, issue-level security permission to view the issue.
- **`jira-cloud-platform-pp-cli bulk submit-watch`** - Use this API to submit a bulk watch request. You can watch up to 1,000 issues in a single operation.

**[Permissions](#permissions) required:**

 *  Global bulk change [permission](https://support.atlassian.com/jira-cloud-administration/docs/manage-global-permissions/).
 *  Browse [project permission](https://support.atlassian.com/jira-cloud-administration/docs/manage-project-permissions/) in all projects that contain the selected issues.
 *  If [issue-level security](https://confluence.atlassian.com/x/J4lKLg) is configured, issue-level security permission to view the issue.

### changelog

Manage changelog

- **`jira-cloud-platform-pp-cli changelog get-bulk`** - Bulk fetch changelogs for multiple issues and filter by fields

Returns a paginated list of all changelogs for given issues sorted by changelog date and issue IDs, starting from the oldest changelog and smallest issue ID.

Issues are identified by their ID or key, and optionally changelogs can be filtered by their field IDs. You can request the changelogs of up to 1000 issues and can filter them by up to 10 field IDs.

**[Permissions](#permissions) required:**

 *  *Browse projects* [project permission](https://confluence.atlassian.com/x/yodKLg) for the projects that the issues are in.
 *  If [issue-level security](https://confluence.atlassian.com/x/J4lKLg) is configured, issue-level security permission to view the issues.

### classification-levels

This resource represents classification levels.

- **`jira-cloud-platform-pp-cli classification-levels get-all-user-data`** - Returns all classification levels.

**[Permissions](#permissions) required:** None.

### comment

Manage comment

- **`jira-cloud-platform-pp-cli comment get-by-ids`** - Returns a [paginated](#pagination) list of comments specified by a list of comment IDs.

This operation can be accessed anonymously.

**[Permissions](#permissions) required:** Comments are returned where the user:

 *  has *Browse projects* [project permission](https://confluence.atlassian.com/x/yodKLg) for the project containing the comment.
 *  If [issue-level security](https://confluence.atlassian.com/x/J4lKLg) is configured, issue-level security permission to view the issue.
 *  If the comment has visibility restrictions, belongs to the group or has the role visibility is restricted to.

### component

Manage component

- **`jira-cloud-platform-pp-cli component create`** - Creates a component. Use components to provide containers for issues within a project. Use components to provide containers for issues within a project.

This operation can be accessed anonymously.

**[Permissions](#permissions) required:** *Administer projects* [project permission](https://confluence.atlassian.com/x/yodKLg) for the project in which the component is created or *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli component delete`** - Deletes a component.

This operation can be accessed anonymously.

**[Permissions](#permissions) required:** *Administer projects* [project permission](https://confluence.atlassian.com/x/yodKLg) for the project containing the component or *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli component find-for-projects`** - Returns a [paginated](#pagination) list of all components in a project, including global (Compass) components when applicable.

This operation can be accessed anonymously.

**[Permissions](#permissions) required:** *Browse Projects* [project permission](https://confluence.atlassian.com/x/yodKLg) for the project.
- **`jira-cloud-platform-pp-cli component get`** - Returns a component.

This operation can be accessed anonymously.

**[Permissions](#permissions) required:** *Browse projects* [project permission](https://confluence.atlassian.com/x/yodKLg) for project containing the component.
- **`jira-cloud-platform-pp-cli component update`** - Updates a component. Any fields included in the request are overwritten. If `leadAccountId` is an empty string ("") the component lead is removed.

This operation can be accessed anonymously.

**[Permissions](#permissions) required:** *Administer projects* [project permission](https://confluence.atlassian.com/x/yodKLg) for the project containing the component or *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).

### config

Manage config

- **`jira-cloud-platform-pp-cli config associate-projects-to-field-association-schemes`** - Associate projects to field association schemes.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli config clone-field-association-scheme`** - Endpoint for cloning an existing field association scheme into a new one.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli config create-field-association-scheme`** - Endpoint for creating a new field association scheme.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli config delete-field-association-scheme`** - Delete a specified field association scheme

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli config get-field-association-scheme-by-id`** - Endpoint for fetching a field association scheme by its ID

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli config get-field-association-scheme-item-parameters`** - Retrieve field association parameters on a field association scheme

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli config get-field-association-schemes`** - REST endpoint for retrieving a paginated list of field association schemes with optional filtering.

This endpoint allows clients to fetch field association schemes with optional filtering by project IDs and text queries. The response includes scheme details with navigation links and filter metadata when applicable.

Filtering Behavior:

 *  When projectId or query parameters are provided, the response includes matchedFilters metadata showing which filters were applied.
 *  When no filters are applied, matchedFilters is omitted from individual scheme objects

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli config get-projects-with-field-schemes`** - Get projects with field association schemes. This will be a temporary API but useful when transitioning from the legacy field configuration APIs to the new ones.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli config remove-field-association-scheme-item-parameters`** - Remove field association parameters overrides for work types.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli config remove-fields-associated-with-schemes`** - Remove fields associated with field association schemes.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli config search-field-association-scheme-fields`** - Search for fields belonging to a given field association scheme.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli config search-field-association-scheme-projects`** - REST Endpoint for searching for projects belonging to a given field association scheme

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli config update-field-association-scheme`** - Endpoint for updating an existing field association scheme.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli config update-field-association-scheme-item-parameters`** - Update field association item parameters in field association schemes.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli config update-fields-associated-with-schemes`** - Update fields associated with field association schemes.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).

### configuration

Manage configuration

- **`jira-cloud-platform-pp-cli configuration get`** - Returns the [global settings](https://confluence.atlassian.com/x/qYXKM) in Jira. These settings determine whether optional features (for example, subtasks, time tracking, and others) are enabled. If time tracking is enabled, this operation also returns the time tracking configuration.

**[Permissions](#permissions) required:** Permission to access Jira.
- **`jira-cloud-platform-pp-cli configuration get-available-time-tracking-implementations`** - Returns all time tracking providers. By default, Jira only has one time tracking provider: *JIRA provided time tracking*. However, you can install other time tracking providers via apps from the Atlassian Marketplace. For more information on time tracking providers, see the documentation for the [ Time Tracking Provider](https://developer.atlassian.com/cloud/jira/platform/modules/time-tracking-provider/) module.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli configuration get-selected-time-tracking-implementation`** - Returns the time tracking provider that is currently selected. Note that if time tracking is disabled, then a successful but empty response is returned.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli configuration get-shared-time-tracking`** - Returns the time tracking settings. This includes settings such as the time format, default time unit, and others. For more information, see [Configuring time tracking](https://confluence.atlassian.com/x/qoXKM).

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli configuration select-time-tracking-implementation`** - Selects a time tracking provider.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli configuration set-shared-time-tracking`** - Sets the time tracking settings.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).

### custom-field-option

Manage custom field option

- **`jira-cloud-platform-pp-cli custom-field-option get`** - Returns a custom field option. For example, an option in a select list.

Note that this operation **only works for issue field select list options created in Jira or using operations from the [Issue custom field options](#api-group-Issue-custom-field-options) resource**, it cannot be used with issue field select list options created by Connect apps.

This operation can be accessed anonymously.

**[Permissions](#permissions) required:** The custom field option is returned as follows:

 *  if the user has the *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
 *  if the user has the *Browse projects* [project permission](https://confluence.atlassian.com/x/yodKLg) for at least one project the custom field is used in, and the field is visible in at least one layout the user has permission to view.

### dashboard

This resource represents dashboards. Use it to obtain the details of dashboards as well as get, create, update, or remove item properties and gadgets from dashboards.

- **`jira-cloud-platform-pp-cli dashboard bulk-edit`** - Bulk edit dashboards. Maximum number of dashboards to be edited at the same time is 100.

**[Permissions](#permissions) required:** None

The dashboards to be updated must be owned by the user, or the user must be an administrator.
- **`jira-cloud-platform-pp-cli dashboard create`** - Creates a dashboard.

**[Permissions](#permissions) required:** None.
- **`jira-cloud-platform-pp-cli dashboard delete`** - Deletes a dashboard.

**[Permissions](#permissions) required:** None

The dashboard to be deleted must be owned by the user.
- **`jira-cloud-platform-pp-cli dashboard get`** - Returns a dashboard.

This operation can be accessed anonymously.

**[Permissions](#permissions) required:** None.

However, to get a dashboard, the dashboard must be shared with the user or the user must own it. Note, users with the *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg) are considered owners of the System dashboard. The System dashboard is considered to be shared with all other users.
- **`jira-cloud-platform-pp-cli dashboard get-all`** - Returns a list of dashboards owned by or shared with the user. The list may be filtered to include only favorite or owned dashboards.

This operation can be accessed anonymously.

**[Permissions](#permissions) required:** None.
- **`jira-cloud-platform-pp-cli dashboard get-all-available-gadgets`** - Gets a list of all available gadgets that can be added to all dashboards.

**[Permissions](#permissions) required:** None.
- **`jira-cloud-platform-pp-cli dashboard get-paginated`** - Returns a [paginated](#pagination) list of dashboards. This operation is similar to [Get dashboards](#api-rest-api-3-dashboard-get) except that the results can be refined to include dashboards that have specific attributes. For example, dashboards with a particular name. When multiple attributes are specified only filters matching all attributes are returned.

This operation can be accessed anonymously.

**[Permissions](#permissions) required:** The following dashboards that match the query parameters are returned:

 *  Dashboards owned by the user. Not returned for anonymous users.
 *  Dashboards shared with a group that the user is a member of. Not returned for anonymous users.
 *  Dashboards shared with a private project that the user can browse. Not returned for anonymous users.
 *  Dashboards shared with a public project.
 *  Dashboards shared with the public.
- **`jira-cloud-platform-pp-cli dashboard update`** - Updates a dashboard, replacing all the dashboard details with those provided.

**[Permissions](#permissions) required:** None

The dashboard to be updated must be owned by the user.

### data-policy

Manage data policy

- **`jira-cloud-platform-pp-cli data-policy get-policies`** - Returns data policies for the projects specified in the request.
- **`jira-cloud-platform-pp-cli data-policy get-policy`** - Returns data policy for the workspace.

### events

Manage events

- **`jira-cloud-platform-pp-cli events get`** - Returns all issue events.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).

### expression

Manage expression

- **`jira-cloud-platform-pp-cli expression analyse`** - Analyses and validates Jira expressions.

As an experimental feature, this operation can also attempt to type-check the expressions.

Learn more about Jira expressions in the [documentation](https://developer.atlassian.com/cloud/jira/platform/jira-expressions/).

**[Permissions](#permissions) required**: None.
- **`jira-cloud-platform-pp-cli expression evaluate-jira`** - Endpoint is currently being removed. [More details](https://developer.atlassian.com/changelog/#CHANGE-2046)

Evaluates a Jira expression and returns its value.

This resource can be used to test Jira expressions that you plan to use elsewhere, or to fetch data in a flexible way. Consult the [Jira expressions documentation](https://developer.atlassian.com/cloud/jira/platform/jira-expressions/) for more details.

#### Context variables ####

The following context variables are available to Jira expressions evaluated by this resource. Their presence depends on various factors; usually you need to manually request them in the context object sent in the payload, but some of them are added automatically under certain conditions.

 *  `user` ([User](https://developer.atlassian.com/cloud/jira/platform/jira-expressions-type-reference#user)): The current user. Always available and equal to `null` if the request is anonymous.
 *  `app` ([App](https://developer.atlassian.com/cloud/jira/platform/jira-expressions-type-reference#app)): The [Connect app](https://developer.atlassian.com/cloud/jira/platform/index/#connect-apps) that made the request. Available only for authenticated requests made by Connect Apps (read more here: [Authentication for Connect apps](https://developer.atlassian.com/cloud/jira/platform/security-for-connect-apps/)).
 *  `issue` ([Issue](https://developer.atlassian.com/cloud/jira/platform/jira-expressions-type-reference#issue)): The current issue. Available only when the issue is provided in the request context object.
 *  `issues` ([List](https://developer.atlassian.com/cloud/jira/platform/jira-expressions-type-reference#list) of [Issues](https://developer.atlassian.com/cloud/jira/platform/jira-expressions-type-reference#issue)): A collection of issues matching a JQL query. Available only when JQL is provided in the request context object.
 *  `project` ([Project](https://developer.atlassian.com/cloud/jira/platform/jira-expressions-type-reference#project)): The current project. Available only when the project is provided in the request context object.
 *  `sprint` ([Sprint](https://developer.atlassian.com/cloud/jira/platform/jira-expressions-type-reference#sprint)): The current sprint. Available only when the sprint is provided in the request context object.
 *  `board` ([Board](https://developer.atlassian.com/cloud/jira/platform/jira-expressions-type-reference#board)): The current board. Available only when the board is provided in the request context object.
 *  `serviceDesk` ([ServiceDesk](https://developer.atlassian.com/cloud/jira/platform/jira-expressions-type-reference#servicedesk)): The current service desk. Available only when the service desk is provided in the request context object.
 *  `customerRequest` ([CustomerRequest](https://developer.atlassian.com/cloud/jira/platform/jira-expressions-type-reference#customerrequest)): The current customer request. Available only when the customer request is provided in the request context object.

Also, custom context variables can be passed in the request with their types. Those variables can be accessed by key in the Jira expression. These variable types are available for use in a custom context:

 *  `user`: A [user](https://developer.atlassian.com/cloud/jira/platform/jira-expressions-type-reference#user) specified as an Atlassian account ID.
 *  `issue`: An [issue](https://developer.atlassian.com/cloud/jira/platform/jira-expressions-type-reference#issue) specified by ID or key. All the fields of the issue object are available in the Jira expression.
 *  `json`: A JSON object containing custom content.
 *  `list`: A JSON list of `user`, `issue`, or `json` variable types.

This operation can be accessed anonymously.

**[Permissions](#permissions) required**: None. However, an expression may return different results for different users depending on their permissions. For example, different users may see different comments on the same issue.  
Permission to access Jira Software is required to access Jira Software context variables (`board` and `sprint`) or fields (for example, `issue.sprint`).
- **`jira-cloud-platform-pp-cli expression evaluate-jsisjira`** - Evaluates a Jira expression and returns its value. The difference between this and `eval` is that this endpoint uses the enhanced search API when evaluating JQL queries. This API is eventually consistent, unlike the strongly consistent `eval` API. This allows for better performance and scalability. In addition, this API's response for JQL evaluation is based on a scrolling view (backed by a `nextPageToken`) instead of a paginated view (backed by `startAt` and `totalCount`).

This resource can be used to test Jira expressions that you plan to use elsewhere, or to fetch data in a flexible way. Consult the [Jira expressions documentation](https://developer.atlassian.com/cloud/jira/platform/jira-expressions/) for more details.

#### Context variables ####

The following context variables are available to Jira expressions evaluated by this resource. Their presence depends on various factors; usually you need to manually request them in the context object sent in the payload, but some of them are added automatically under certain conditions.

 *  `user` ([User](https://developer.atlassian.com/cloud/jira/platform/jira-expressions-type-reference#user)): The current user. Always available and equal to `null` if the request is anonymous.
 *  `app` ([App](https://developer.atlassian.com/cloud/jira/platform/jira-expressions-type-reference#app)): The [Connect app](https://developer.atlassian.com/cloud/jira/platform/index/#connect-apps) that made the request. Available only for authenticated requests made by Connect apps (read more here: [Authentication for Connect apps](https://developer.atlassian.com/cloud/jira/platform/security-for-connect-apps/)).
 *  `issue` ([Issue](https://developer.atlassian.com/cloud/jira/platform/jira-expressions-type-reference#issue)): The current issue. Available only when the issue is provided in the request context object.
 *  `issues` ([List](https://developer.atlassian.com/cloud/jira/platform/jira-expressions-type-reference#list) of [Issues](https://developer.atlassian.com/cloud/jira/platform/jira-expressions-type-reference#issue)): A collection of issues matching a JQL query. Available only when JQL is provided in the request context object.
 *  `project` ([Project](https://developer.atlassian.com/cloud/jira/platform/jira-expressions-type-reference#project)): The current project. Available only when the project is provided in the request context object.
 *  `sprint` ([Sprint](https://developer.atlassian.com/cloud/jira/platform/jira-expressions-type-reference#sprint)): The current sprint. Available only when the sprint is provided in the request context object.
 *  `board` ([Board](https://developer.atlassian.com/cloud/jira/platform/jira-expressions-type-reference#board)): The current board. Available only when the board is provided in the request context object.
 *  `serviceDesk` ([ServiceDesk](https://developer.atlassian.com/cloud/jira/platform/jira-expressions-type-reference#servicedesk)): The current service desk. Available only when the service desk is provided in the request context object.
 *  `customerRequest` ([CustomerRequest](https://developer.atlassian.com/cloud/jira/platform/jira-expressions-type-reference#customerrequest)): The current customer request. Available only when the customer request is provided in the request context object.

In addition, you can pass custom context variables along with their types. You can then access them from the Jira expression by key. You can use the following variables in a custom context:

 *  `user`: A [user](https://developer.atlassian.com/cloud/jira/platform/jira-expressions-type-reference#user) specified as an Atlassian account ID.
 *  `issue`: An [issue](https://developer.atlassian.com/cloud/jira/platform/jira-expressions-type-reference#issue) specified by ID or key. All the fields of the issue object are available in the Jira expression.
 *  `json`: A JSON object containing custom content.
 *  `list`: A JSON list of `user`, `issue`, or `json` variable types.

This operation can be accessed anonymously.

**[Permissions](#permissions) required**: None. However, an expression may return different results for different users depending on their permissions. For example, different users may see different comments on the same issue.  
Permission to access Jira Software is required to access Jira Software context variables (`board` and `sprint`) or fields (for example, `issue.sprint`).

### field

Manage field

- **`jira-cloud-platform-pp-cli field create-associations`** - Associates fields with projects.

Fields will be associated with each issue type on the requested projects.

Fields will be associated with all projects that share the same field configuration which the provided projects are using. This means that while the field will be associated with the requested projects, it will also be associated with any other projects that share the same field configuration.

If a success response is returned it means that the field association has been created in any applicable contexts where it wasn't already present.

Up to 50 fields and up to 100 projects can be associated in a single request. If more fields or projects are provided a 400 response will be returned.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli field create-custom`** - Creates a custom field.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli field delete-custom`** - Deletes a custom field. The custom field is deleted whether it is in the trash or not. See [Edit or delete a custom field](https://confluence.atlassian.com/x/Z44fOw) for more information on trashing and deleting custom fields.

This operation is [asynchronous](#async). Follow the `location` link in the response to determine the status of the task and use [Get task](#api-rest-api-3-task-taskId-get) to obtain subsequent updates.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli field get`** - Returns system and custom issue fields according to the following rules:

 *  Fields that cannot be added to the issue navigator are always returned.
 *  Fields that cannot be placed on an issue screen are always returned.
 *  Fields that depend on global Jira settings are only returned if the setting is enabled. That is, timetracking fields, subtasks, votes, and watches.
 *  Fields that are not associated to any used field configurations or screens are not returned.
 *  For all other fields, this operation only returns the fields that the user has permission to view (that is, the field is used in at least one project that the user has *Browse Projects* [project permission](https://confluence.atlassian.com/x/yodKLg) for.)

This operation can be accessed anonymously.

**[Permissions](#permissions) required:** None.
- **`jira-cloud-platform-pp-cli field get-paginated`** - Returns a [paginated](#pagination) list of fields for Classic Jira projects. The list can include:

 *  all fields
 *  specific fields, by defining `id`
 *  fields that contain a string in the field name or description, by defining `query`
 *  specific fields that contain a string in the field name or description, by defining `id` and `query`

Use `type` must be set to `custom` to show custom fields only.

**[Permissions](#permissions) required:** Permission to access Jira.
- **`jira-cloud-platform-pp-cli field get-trashed-paginated`** - Returns a [paginated](#pagination) list of fields in the trash. The list may be restricted to fields whose field name or description partially match a string.

Only custom fields can be queried, `type` must be set to `custom`.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli field remove-associations`** - Unassociates a set of fields with a project and issue type context.

Fields will be unassociated with all projects/issue types that share the same field configuration which the provided project and issue types are using. This means that while the field will be unassociated with the provided project and issue types, it will also be unassociated with any other projects and issue types that share the same field configuration.

If a success response is returned it means that the field association has been removed in any applicable contexts where it was present.

Up to 50 fields and up to 100 projects and issue types can be unassociated in a single request. If more fields or projects are provided a 400 response will be returned.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli field update-custom`** - Updates a custom field.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).

### fieldconfiguration

Manage fieldconfiguration

- **`jira-cloud-platform-pp-cli fieldconfiguration create-field-configuration`** - Deprecated, use [ Field schemes](https://developer.atlassian.com/cloud/jira/platform/rest/v3/api-group-field-schemes/#api-group-field-schemes) which supports field association schemes.

Creates a field configuration. The field configuration is created with the same field properties as the default configuration, with all the fields being optional.

This operation can only create configurations for use in company-managed (classic) projects.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli fieldconfiguration delete-field-configuration`** - Deprecated, use [ Field schemes](https://developer.atlassian.com/cloud/jira/platform/rest/v3/api-group-field-schemes/#api-group-field-schemes) which supports field association schemes.

Deletes a field configuration.

This operation can only delete configurations used in company-managed (classic) projects.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli fieldconfiguration get-all-field-configurations`** - Deprecated, use [ Field schemes](https://developer.atlassian.com/cloud/jira/platform/rest/v3/api-group-field-schemes/#api-group-field-schemes) which supports field association schemes.

Returns a [paginated](#pagination) list of field configurations. The list can be for all field configurations or a subset determined by any combination of these criteria:

 *  a list of field configuration item IDs.
 *  whether the field configuration is a default.
 *  whether the field configuration name or description contains a query string.

Only field configurations used in company-managed (classic) projects are returned.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli fieldconfiguration update-field-configuration`** - Deprecated, use [ Field schemes](https://developer.atlassian.com/cloud/jira/platform/rest/v3/api-group-field-schemes/#api-group-field-schemes) which supports field association schemes.

Updates a field configuration. The name and the description provided in the request override the existing values.

This operation can only update configurations used in company-managed (classic) projects.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).

### fieldconfigurationscheme

Manage fieldconfigurationscheme

- **`jira-cloud-platform-pp-cli fieldconfigurationscheme assign-field-configuration-scheme-to-project`** - Deprecated, use [ Field schemes](https://developer.atlassian.com/cloud/jira/platform/rest/v3/api-group-field-schemes/#api-group-field-schemes) which supports field association schemes.

Assigns a field configuration scheme to a project. If the field configuration scheme ID is `null`, the operation assigns the default field configuration scheme.

Field configuration schemes can only be assigned to classic projects.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli fieldconfigurationscheme create-field-configuration-scheme`** - Deprecated, use [ Field schemes](https://developer.atlassian.com/cloud/jira/platform/rest/v3/api-group-field-schemes/#api-group-field-schemes) which supports field association schemes.

Creates a field configuration scheme.

This operation can only create field configuration schemes used in company-managed (classic) projects.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli fieldconfigurationscheme delete-field-configuration-scheme`** - Deprecated, use [ Field schemes](https://developer.atlassian.com/cloud/jira/platform/rest/v3/api-group-field-schemes/#api-group-field-schemes) which supports field association schemes.

Deletes a field configuration scheme.

This operation can only delete field configuration schemes used in company-managed (classic) projects.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli fieldconfigurationscheme get-all-field-configuration-schemes`** - Deprecated, use [ Field schemes](https://developer.atlassian.com/cloud/jira/platform/rest/v3/api-group-field-schemes/#api-group-field-schemes) which supports field association schemes.

Returns a [paginated](#pagination) list of field configuration schemes.

Only field configuration schemes used in classic projects are returned.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli fieldconfigurationscheme get-field-configuration-scheme-mappings`** - Deprecated, use [ Field schemes](https://developer.atlassian.com/cloud/jira/platform/rest/v3/api-group-field-schemes/#api-group-field-schemes) which supports field association schemes.

Returns a [paginated](#pagination) list of field configuration issue type items.

Only items used in classic projects are returned.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli fieldconfigurationscheme get-field-configuration-scheme-project-mapping`** - Deprecated, use [ Field schemes](https://developer.atlassian.com/cloud/jira/platform/rest/v3/api-group-field-schemes/#api-group-field-schemes) which supports field association schemes.

Returns a [paginated](#pagination) list of field configuration schemes and, for each scheme, a list of the projects that use it.

The list is sorted by field configuration scheme ID. The first item contains the list of project IDs assigned to the default field configuration scheme.

Only field configuration schemes used in classic projects are returned.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli fieldconfigurationscheme update-field-configuration-scheme`** - Deprecated, use [ Field schemes](https://developer.atlassian.com/cloud/jira/platform/rest/v3/api-group-field-schemes/#api-group-field-schemes) which supports field association schemes.

Updates a field configuration scheme.

This operation can only update field configuration schemes used in company-managed (classic) projects.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).

### filter

This resource represents [filters](https://confluence.atlassian.com/x/eQiiLQ). Use it to get, create, update, or delete filters. Also use it to configure the columns for a filter and set favorite filters.

- **`jira-cloud-platform-pp-cli filter create`** - Creates a filter. The filter is shared according to the [default share scope](#api-rest-api-3-filter-post). The filter is not selected as a favorite.

**[Permissions](#permissions) required:** Permission to access Jira.
- **`jira-cloud-platform-pp-cli filter delete`** - Delete a filter.

**[Permissions](#permissions) required:** Permission to access Jira, however filters can only be deleted by the creator of the filter or a user with *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli filter get`** - Returns a filter.

This operation can be accessed anonymously.

**[Permissions](#permissions) required:** None, however, the filter is only returned where it is:

 *  owned by the user.
 *  shared with a group that the user is a member of.
 *  shared with a private project that the user has *Browse projects* [project permission](https://confluence.atlassian.com/x/yodKLg) for.
 *  shared with a public project.
 *  shared with the public.
- **`jira-cloud-platform-pp-cli filter get-default-share-scope`** - Returns the default sharing settings for new filters and dashboards for a user.

**[Permissions](#permissions) required:** Permission to access Jira.
- **`jira-cloud-platform-pp-cli filter get-favourite`** - Returns the visible favorite filters of the user.

This operation can be accessed anonymously.

**[Permissions](#permissions) required:** A favorite filter is only visible to the user where the filter is:

 *  owned by the user.
 *  shared with a group that the user is a member of.
 *  shared with a private project that the user has *Browse projects* [project permission](https://confluence.atlassian.com/x/yodKLg) for.
 *  shared with a public project.
 *  shared with the public.

For example, if the user favorites a public filter that is subsequently made private that filter is not returned by this operation.
- **`jira-cloud-platform-pp-cli filter get-my`** - Returns the filters owned by the user. If `includeFavourites` is `true`, the user's visible favorite filters are also returned.

**[Permissions](#permissions) required:** Permission to access Jira, however, a favorite filters is only visible to the user where the filter is:

 *  owned by the user.
 *  shared with a group that the user is a member of.
 *  shared with a private project that the user has *Browse projects* [project permission](https://confluence.atlassian.com/x/yodKLg) for.
 *  shared with a public project.
 *  shared with the public.

For example, if the user favorites a public filter that is subsequently made private that filter is not returned by this operation.
- **`jira-cloud-platform-pp-cli filter get-paginated`** - Returns a [paginated](#pagination) list of filters. Use this operation to get:

 *  specific filters, by defining `id` only.
 *  filters that match all of the specified attributes. For example, all filters for a user with a particular word in their name. When multiple attributes are specified only filters matching all attributes are returned.

This operation can be accessed anonymously.

**[Permissions](#permissions) required:** None, however, only the following filters that match the query parameters are returned:

 *  filters owned by the user.
 *  filters shared with a group that the user is a member of.
 *  filters shared with a private project that the user has *Browse projects* [project permission](https://confluence.atlassian.com/x/yodKLg) for.
 *  filters shared with a public project.
 *  filters shared with the public.
- **`jira-cloud-platform-pp-cli filter set-default-share-scope`** - Sets the default sharing for new filters and dashboards for a user.

**[Permissions](#permissions) required:** Permission to access Jira.
- **`jira-cloud-platform-pp-cli filter update`** - Updates a filter. Use this operation to update a filter's name, description, JQL, or sharing.

**[Permissions](#permissions) required:** Permission to access Jira, however the user must own the filter.

### forge

Manage forge

- **`jira-cloud-platform-pp-cli forge bulk-pin-unpin-projects-async`** - Bulk pin or unpin an issue panel (added by a Forge app) to or from multiple projects.

The operation runs asynchronously. The response includes a task ID - use the [Get task](#api-rest-api-3-task-taskId-get) endpoint to check progress.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli forge delete-app-property`** - Deletes a Forge app's property.

**[Permissions](#permissions) required:** Only Forge apps can make this request. This API can only be accessed using **[asApp()](https://developer.atlassian.com/platform/forge/apis-reference/fetch-api-product.requestjira/#method-signature)** requests from Forge.

The new `write:app-data:jira` OAuth scope is 100% optional now, and not using it won't break your app. However, we recommend adding it to your app's scope list because we will eventually make it mandatory.
- **`jira-cloud-platform-pp-cli forge get-app-property`** - Returns the value of a Forge app's property.

**[Permissions](#permissions) required:** Only Forge apps can make this request. This API can only be accessed using **[asApp()](https://developer.atlassian.com/platform/forge/apis-reference/fetch-api-product.requestjira/#method-signature)** requests from Forge.
- **`jira-cloud-platform-pp-cli forge get-app-property-keys`** - Returns all property keys for the Forge app.

**[Permissions](#permissions) required:** Only Forge apps can make this request. This API can only be accessed using **[asApp()](https://developer.atlassian.com/platform/forge/apis-reference/fetch-api-product.requestjira/#method-signature)** requests from Forge.
- **`jira-cloud-platform-pp-cli forge put-app-property`** - Sets the value of a Forge app's property.
These values can be retrieved in [Jira expressions](/cloud/jira/platform/jira-expressions/)
through the `app` [context variable](/cloud/jira/platform/jira-expressions/#context-variables).
They are also available in [entity property display conditions](/platform/forge/manifest-reference/display-conditions/entity-property-conditions/).

For other use cases, use the [Storage API](/platform/forge/runtime-reference/storage-api/).

The value of the request body must be a [valid](http://tools.ietf.org/html/rfc4627), non-empty JSON blob. The maximum length is 32768 characters.

**[Permissions](#permissions) required:** Only Forge apps can make this request. This API can only be accessed using **[asApp()](https://developer.atlassian.com/platform/forge/apis-reference/fetch-api-product.requestjira/#method-signature)** requests from Forge.

The new `write:app-data:jira` OAuth scope is 100% optional now, and not using it won't break your app. However, we recommend adding it to your app's scope list because we will eventually make it mandatory.

### group

This resource represents groups of users. Use it to get, create, find, and delete groups as well as add and remove users from groups. (\[WARNING\] The standard Atlassian group names are default names only and can be edited or deleted. For example, an admin or Atlassian support could delete the default group jira-software-users or rename it to jsw-users at any point. See https://support.atlassian.com/user-management/docs/create-and-update-groups/ for details.)

- **`jira-cloud-platform-pp-cli group add-user-to`** - Adds a user to a group.

**[Permissions](#permissions) required:** Site administration (that is, member of the *site-admin* [group](https://confluence.atlassian.com/x/24xjL)).
- **`jira-cloud-platform-pp-cli group bulk-get`** - Returns a [paginated](#pagination) list of groups.

**[Permissions](#permissions) required:** *Browse users and groups* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli group create`** - Creates a group.

**[Permissions](#permissions) required:** Site administration (that is, member of the *site-admin* [group](https://confluence.atlassian.com/x/24xjL)).
- **`jira-cloud-platform-pp-cli group get`** - This operation is deprecated, use [`group/member`](#api-rest-api-3-group-member-get).

Returns all users in a group.

**[Permissions](#permissions) required:** either of:

 *  *Browse users and groups* [global permission](https://confluence.atlassian.com/x/x4dKLg).
 *  *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli group get-users-from`** - Returns a [paginated](#pagination) list of all users in a group.

Note that users are ordered by username, however the username is not returned in the results due to privacy reasons.

**[Permissions](#permissions) required:** either of:

 *  *Browse users and groups* [global permission](https://confluence.atlassian.com/x/x4dKLg).
 *  *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli group remove`** - Deletes a group.

**[Permissions](#permissions) required:** Site administration (that is, member of the *site-admin* strategic [group](https://confluence.atlassian.com/x/24xjL)).
- **`jira-cloud-platform-pp-cli group remove-user-from`** - Removes a user from a group.

**[Permissions](#permissions) required:** Site administration (that is, member of the *site-admin* [group](https://confluence.atlassian.com/x/24xjL)).

### groups

This resource represents groups of users. Use it to get, create, find, and delete groups as well as add and remove users from groups. (\[WARNING\] The standard Atlassian group names are default names only and can be edited or deleted. For example, an admin or Atlassian support could delete the default group jira-software-users or rename it to jsw-users at any point. See https://support.atlassian.com/user-management/docs/create-and-update-groups/ for details.)

- **`jira-cloud-platform-pp-cli groups find`** - Returns a list of groups whose names contain a query string. A list of group names can be provided to exclude groups from the results.

The primary use case for this resource is to populate a group picker suggestions list. To this end, the returned object includes the `html` field where the matched query term is highlighted in the group name with the HTML strong tag. Also, the groups list is wrapped in a response object that contains a header for use in the picker, specifically *Showing X of Y matching groups*.

The list returns with the groups sorted. If no groups match the list criteria, an empty list is returned.

This operation can be accessed anonymously.

**[Permissions](#permissions) required:** *Browse projects* [project permission](https://confluence.atlassian.com/x/yodKLg). Anonymous calls and calls by users without the required permission return an empty list.

*Browse users and groups* [global permission](https://confluence.atlassian.com/x/x4dKLg). Without this permission, calls where query is not an exact match to an existing group will return an empty list.

### groupuserpicker

Manage groupuserpicker

- **`jira-cloud-platform-pp-cli groupuserpicker find-users-and-groups`** - Returns a list of users and groups matching a string. The string is used:

 *  for users, to find a case-insensitive match with display name and e-mail address. Note that if a user has hidden their email address in their user profile, partial matches of the email address will not find the user. An exact match is required.
 *  for groups, to find a case-sensitive match with group name.

For example, if the string *tin* is used, records with the display name *Tina*, email address matching the query, and the group *accounting* would be returned.

Optionally, the search can be refined to:

 *  the projects and issue types associated with a custom field, such as a user picker. The search can then be further refined to return only users and groups that have permission to view specific:
    
     *  projects.
     *  issue types.
    
    If multiple projects or issue types are specified, they must be a subset of those enabled for the custom field or no results are returned. For example, if a field is enabled for projects A, B, and C then the search could be limited to projects B and C. However, if the search is limited to projects B and D, nothing is returned.
 *  not return Connect app users and groups.
 *  return groups that have a case-insensitive match with the query.

The primary use case for this resource is to populate a picker field suggestion list with users or groups. To this end, the returned object includes an `html` field for each list. This field highlights the matched query term in the item name with the HTML strong tag. Also, each list is wrapped in a response object that contains a header for use in a picker, specifically *Showing X of Y matching groups*.

This operation can be accessed anonymously.

**[Permissions](#permissions) required:** *Browse users and groups* [global permission](https://confluence.atlassian.com/x/yodKLg).

### instance

Manage instance

- **`jira-cloud-platform-pp-cli instance get-license`** - Returns licensing information about the Jira instance.

**[Permissions](#permissions) required:** None.

### internal

Manage internal

- **`jira-cloud-platform-pp-cli internal get-worklogs-by-issue-id-and-worklog-id`** - Returns worklog details for a list of issue ID and worklog ID pairs.

This is an internal API for bulk fetching worklogs by their issue and worklog IDs. Worklogs that don't exist will be filtered out from the response.

The returned list of worklogs is limited to 1000 items.

**[Permissions](#permissions) required:** This is an internal service-to-service API that requires ASAP authentication. No user permission checks are performed as this bypasses normal user context.

### issue

This resource represents Jira issues. Use it to:

 *  create or edit issues, individually or in bulk.
 *  retrieve metadata about the options for creating or editing issues.
 *  delete an issue.
 *  assign a user to an issue.
 *  get issue changelogs.
 *  send notifications about an issue.
 *  get details of the transitions available for an issue.
 *  transition an issue.
 *  Archive issues.
 *  Unarchive issues.
 *  Export archived issues.

- **`jira-cloud-platform-pp-cli issue archive`** - Enables admins to archive up to 1000 issues in a single request using issue ID/key, returning details of the issue(s) archived in the process and the errors encountered, if any.

**Note that:**

 *  you can't archive subtasks directly, only through their parent issues
 *  you can only archive issues from software, service management, and business projects

**[Permissions](#permissions) required:** Jira admin or site admin: [global permission](https://confluence.atlassian.com/x/x4dKLg)

**License required:** Premium or Enterprise

**Signed-in users only:** This API can't be accessed anonymously.
- **`jira-cloud-platform-pp-cli issue archive-async`** - Enables admins to archive up to 100,000 issues in a single request using JQL, returning the URL to check the status of the submitted request.

You can use the [get task](https://developer.atlassian.com/cloud/jira/platform/rest/v3/api-group-tasks/#api-rest-api-3-task-taskid-get) and [cancel task](https://developer.atlassian.com/cloud/jira/platform/rest/v3/api-group-tasks/#api-rest-api-3-task-taskid-cancel-post) APIs to manage the request.

**Note that:**

 *  you can't archive subtasks directly, only through their parent issues
 *  you can only archive issues from software, service management, and business projects

**[Permissions](#permissions) required:** Jira admin or site admin: [global permission](https://confluence.atlassian.com/x/x4dKLg)

**License required:** Premium or Enterprise

**Signed-in users only:** This API can't be accessed anonymously.

**Rate limiting:** Only a single request per jira instance can be active at any given time.
- **`jira-cloud-platform-pp-cli issue bulk-delete-property`** - Deletes a property value from multiple issues. The issues to be updated can be specified by filter criteria.

The criteria the filter used to identify eligible issues are:

 *  `entityIds` Only issues from this list are eligible.
 *  `currentValue` Only issues with the property set to this value are eligible.

If both criteria is specified, they are joined with the logical *AND*: only issues that satisfy both criteria are considered eligible.

If no filter criteria are specified, all the issues visible to the user and where the user has the EDIT\_ISSUES permission for the issue are considered eligible.

This operation is:

 *  transactional, either the property is deleted from all eligible issues or, when errors occur, no properties are deleted.
 *  [asynchronous](#async). Follow the `location` link in the response to determine the status of the task and use [Get task](#api-rest-api-3-task-taskId-get) to obtain subsequent updates.

**[Permissions](#permissions) required:**

 *  *Browse projects* [ project permission](https://confluence.atlassian.com/x/yodKLg) for each project containing issues.
 *  If [issue-level security](https://confluence.atlassian.com/x/J4lKLg) is configured, issue-level security permission to view the issue.
 *  *Edit issues* [project permission](https://confluence.atlassian.com/x/yodKLg) for each issue.
- **`jira-cloud-platform-pp-cli issue bulk-fetch`** - Returns the details for a set of requested issues. You can request up to 100 issues.

Each issue is identified by its ID or key, however, if the identifier doesn't match an issue, a case-insensitive search and check for moved issues is performed. If a matching issue is found its details are returned, a 302 or other redirect is **not** returned.

Issues will be returned in ascending `id` order. If there are errors, Jira will return a list of issues which couldn't be fetched along with error messages.

This operation can be accessed anonymously.

**[Permissions](#permissions) required:** Issues are included in the response where the user has:

 *  *Browse projects* [project permission](https://confluence.atlassian.com/x/yodKLg) for the project that the issue is in.
 *  If [issue-level security](https://confluence.atlassian.com/x/J4lKLg) is configured, issue-level security permission to view the issue.
- **`jira-cloud-platform-pp-cli issue bulk-set-properties-by`** - Sets or updates entity property values on issues. Up to 10 entity properties can be specified for each issue and up to 100 issues included in the request.

The value of the request body must be a [valid](http://tools.ietf.org/html/rfc4627), non-empty JSON.

This operation is:

 *  [asynchronous](#async). Follow the `location` link in the response to determine the status of the task and use [Get task](#api-rest-api-3-task-taskId-get) to obtain subsequent updates.
 *  non-transactional. Updating some entities may fail. Such information will available in the task result.

**[Permissions](#permissions) required:**

 *  *Browse projects* and *Edit issues* [project permissions](https://confluence.atlassian.com/x/yodKLg) for the project containing the issue.
 *  If [issue-level security](https://confluence.atlassian.com/x/J4lKLg) is configured, issue-level security permission to view the issue.
- **`jira-cloud-platform-pp-cli issue bulk-set-properties-list`** - Sets or updates a list of entity property values on issues. A list of up to 10 entity properties can be specified along with up to 10,000 issues on which to set or update that list of entity properties.

The value of the request body must be a [valid](http://tools.ietf.org/html/rfc4627), non-empty JSON. The maximum length of single issue property value is 32768 characters. This operation can be accessed anonymously.

This operation is:

 *  transactional, either all properties are updated in all eligible issues or, when errors occur, no properties are updated.
 *  [asynchronous](#async). Follow the `location` link in the response to determine the status of the task and use [Get task](#api-rest-api-3-task-taskId-get) to obtain subsequent updates.

**[Permissions](#permissions) required:**

 *  *Browse projects* and *Edit issues* [project permissions](https://confluence.atlassian.com/x/yodKLg) for the project containing the issue.
 *  If [issue-level security](https://confluence.atlassian.com/x/J4lKLg) is configured, issue-level security permission to view the issue.
- **`jira-cloud-platform-pp-cli issue bulk-set-property`** - Sets a property value on multiple issues.

The value set can be a constant or determined by a [Jira expression](https://developer.atlassian.com/cloud/jira/platform/jira-expressions/). Expressions must be computable with constant complexity when applied to a set of issues. Expressions must also comply with the [restrictions](https://developer.atlassian.com/cloud/jira/platform/jira-expressions/#restrictions) that apply to all Jira expressions.

The issues to be updated can be specified by a filter.

The filter identifies issues eligible for update using these criteria:

 *  `entityIds` Only issues from this list are eligible.
 *  `currentValue` Only issues with the property set to this value are eligible.
 *  `hasProperty`:
    
     *  If *true*, only issues with the property are eligible.
     *  If *false*, only issues without the property are eligible.

If more than one criteria is specified, they are joined with the logical *AND*: only issues that satisfy all criteria are eligible.

If an invalid combination of criteria is provided, an error is returned. For example, specifying a `currentValue` and `hasProperty` as *false* would not match any issues (because without the property the property cannot have a value).

The filter is optional. Without the filter all the issues visible to the user and where the user has the EDIT\_ISSUES permission for the issue are considered eligible.

This operation is:

 *  transactional, either all eligible issues are updated or, when errors occur, none are updated.
 *  [asynchronous](#async). Follow the `location` link in the response to determine the status of the task and use [Get task](#api-rest-api-3-task-taskId-get) to obtain subsequent updates.

**[Permissions](#permissions) required:**

 *  *Browse projects* [project permission](https://confluence.atlassian.com/x/yodKLg) for each project containing issues.
 *  If [issue-level security](https://confluence.atlassian.com/x/J4lKLg) is configured, issue-level security permission to view the issue.
 *  *Edit issues* [project permission](https://confluence.atlassian.com/x/yodKLg) for each issue.
- **`jira-cloud-platform-pp-cli issue create`** - Creates an issue or, where the option to create subtasks is enabled in Jira, a subtask. A transition may be applied, to move the issue or subtask to a workflow step other than the default start step, and issue properties set.

The content of the issue or subtask is defined using `update` and `fields`. The fields that can be set in the issue or subtask are determined using the [ Get create issue metadata](#api-rest-api-3-issue-createmeta-get). These are the same fields that appear on the issue's create screen. Note that the `description`, `environment`, and any `textarea` type custom fields (multi-line text fields) take Atlassian Document Format content. Single line custom fields (`textfield`) accept a string and don't handle Atlassian Document Format content.

Creating a subtask differs from creating an issue as follows:

 *  `issueType` must be set to a subtask issue type (use [ Get create issue metadata](#api-rest-api-3-issue-createmeta-get) to find subtask issue types).
 *  `parent` must contain the ID or key of the parent issue.

In a next-gen project any issue may be made a child providing that the parent and child are members of the same project.

**[Permissions](#permissions) required:** *Browse projects* and *Create issues* [project permissions](https://confluence.atlassian.com/x/yodKLg) for the project in which the issue or subtask is created.
- **`jira-cloud-platform-pp-cli issue create-bulk`** - Creates upto **50** issues and, where the option to create subtasks is enabled in Jira, subtasks. Transitions may be applied, to move the issues or subtasks to a workflow step other than the default start step, and issue properties set.

The content of each issue or subtask is defined using `update` and `fields`. The fields that can be set in the issue or subtask are determined using the [ Get create issue metadata](#api-rest-api-3-issue-createmeta-get). These are the same fields that appear on the issues' create screens. Note that the `description`, `environment`, and any `textarea` type custom fields (multi-line text fields) take Atlassian Document Format content. Single line custom fields (`textfield`) accept a string and don't handle Atlassian Document Format content.

Creating a subtask differs from creating an issue as follows:

 *  `issueType` must be set to a subtask issue type (use [ Get create issue metadata](#api-rest-api-3-issue-createmeta-get) to find subtask issue types).
 *  `parent` the must contain the ID or key of the parent issue.

**[Permissions](#permissions) required:** *Browse projects* and *Create issues* [project permissions](https://confluence.atlassian.com/x/yodKLg) for the project in which each issue or subtask is created.
- **`jira-cloud-platform-pp-cli issue delete`** - Deletes an issue.

An issue cannot be deleted if it has one or more subtasks. To delete an issue with subtasks, set `deleteSubtasks`. This causes the issue's subtasks to be deleted with the issue.

This operation can be accessed anonymously.

**[Permissions](#permissions) required:**

 *  *Browse projects* and *Delete issues* [project permission](https://confluence.atlassian.com/x/yodKLg) for the project containing the issue.
 *  If [issue-level security](https://confluence.atlassian.com/x/J4lKLg) is configured, issue-level security permission to view the issue.
- **`jira-cloud-platform-pp-cli issue edit`** - Edits an issue. Issue properties may be updated as part of the edit. Please note that issue transition is not supported and is ignored here. To transition an issue, please use [Transition issue](#api-rest-api-3-issue-issueIdOrKey-transitions-post).

The edits to the issue's fields are defined using `update` and `fields`. The fields that can be edited are determined using [ Get edit issue metadata](#api-rest-api-3-issue-issueIdOrKey-editmeta-get).

The parent field may be set by key or ID. For standard issue types, the parent may be removed by setting `update.parent.set.none` to *true*. Note that the `description`, `environment`, and any `textarea` type custom fields (multi-line text fields) take Atlassian Document Format content. Single line custom fields (`textfield`) accept a string and don't handle Atlassian Document Format content.

Connect apps having an app user with *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg), and Forge apps acting on behalf of users with *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg), can override the screen security configuration using `overrideScreenSecurity` and `overrideEditableFlag`.

This operation can be accessed anonymously.

**[Permissions](#permissions) required:**

 *  *Browse projects* and *Edit issues* [project permission](https://confluence.atlassian.com/x/yodKLg) for the project that the issue is in.
 *  If [issue-level security](https://confluence.atlassian.com/x/J4lKLg) is configured, issue-level security permission to view the issue.
- **`jira-cloud-platform-pp-cli issue get`** - Returns the details for an issue.

The issue is identified by its ID or key, however, if the identifier doesn't match an issue, a case-insensitive search and check for moved issues is performed. If a matching issue is found its details are returned, a 302 or other redirect is **not** returned. The issue key returned in the response is the key of the issue found.

This operation can be accessed anonymously.

**[Permissions](#permissions) required:**

 *  *Browse projects* [project permission](https://confluence.atlassian.com/x/yodKLg) for the project that the issue is in.
 *  If [issue-level security](https://confluence.atlassian.com/x/J4lKLg) is configured, issue-level security permission to view the issue.
- **`jira-cloud-platform-pp-cli issue get-create-meta`** - Returns details of projects, issue types within projects, and, when requested, the create screen fields for each issue type for the user. Use the information to populate the requests in [ Create issue](#api-rest-api-3-issue-post) and [Create issues](#api-rest-api-3-issue-bulk-post).

Deprecated, see [Create Issue Meta Endpoint Deprecation Notice](https://developer.atlassian.com/cloud/jira/platform/changelog/#CHANGE-1304).

The request can be restricted to specific projects or issue types using the query parameters. The response will contain information for the valid projects, issue types, or project and issue type combinations requested. Note that invalid project, issue type, or project and issue type combinations do not generate errors.

This operation can be accessed anonymously.

**[Permissions](#permissions) required:** *Create issues* [project permission](https://confluence.atlassian.com/x/yodKLg) in the requested projects.
- **`jira-cloud-platform-pp-cli issue get-create-meta-type-id`** - Returns a page of field metadata for a specified project and issuetype id. Use the information to populate the requests in [ Create issue](#api-rest-api-3-issue-post) and [Create issues](#api-rest-api-3-issue-bulk-post).

This operation can be accessed anonymously.

**[Permissions](#permissions) required:** *Create issues* [project permission](https://confluence.atlassian.com/x/yodKLg) in the requested projects.
- **`jira-cloud-platform-pp-cli issue get-create-meta-types`** - Returns a page of issue type metadata for a specified project. Use the information to populate the requests in [ Create issue](#api-rest-api-3-issue-post) and [Create issues](#api-rest-api-3-issue-bulk-post).

This operation can be accessed anonymously.

**[Permissions](#permissions) required:** *Create issues* [project permission](https://confluence.atlassian.com/x/yodKLg) in the requested projects.
- **`jira-cloud-platform-pp-cli issue get-is-watching-bulk`** - Returns, for the user, details of the watched status of issues from a list. If an issue ID is invalid, the returned watched status is `false`.

This operation requires the **Allow users to watch issues** option to be *ON*. This option is set in General configuration for Jira. See [Configuring Jira application options](https://confluence.atlassian.com/x/uYXKM) for details.

**[Permissions](#permissions) required:**

 *  *Browse projects* [project permission](https://confluence.atlassian.com/x/yodKLg) for the project that the issue is in
 *  If [issue-level security](https://confluence.atlassian.com/x/J4lKLg) is configured, issue-level security permission to view the issue.
- **`jira-cloud-platform-pp-cli issue get-limit-report`** - Returns all issues breaching and approaching per-issue limits.

**[Permissions](#permissions) required:**

 *  *Browse projects* [project permission](https://confluence.atlassian.com/x/yodKLg) is required for the project the issues are in. Results may be incomplete otherwise
 *  *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli issue get-picker-resource`** - Returns lists of issues matching a query string. Use this resource to provide auto-completion suggestions when the user is looking for an issue using a word or string.

This operation returns two lists:

 *  `History Search` which includes issues from the user's history of created, edited, or viewed issues that contain the string in the `query` parameter.
 *  `Current Search` which includes issues that match the JQL expression in `currentJQL` and contain the string in the `query` parameter.

This operation can be accessed anonymously.

**[Permissions](#permissions) required:** None.
- **`jira-cloud-platform-pp-cli issue unarchive`** - Enables admins to unarchive up to 1000 issues in a single request using issue ID/key, returning details of the issue(s) unarchived in the process and the errors encountered, if any.

**Note that:**

 *  you can't unarchive subtasks directly, only through their parent issues
 *  you can only unarchive issues from software, service management, and business projects

**[Permissions](#permissions) required:** Jira admin or site admin: [global permission](https://confluence.atlassian.com/x/x4dKLg)

**License required:** Premium or Enterprise

**Signed-in users only:** This API can't be accessed anonymously.

### issue-link

This resource represents links between issues. Use it to get, create, and delete links between issues.

To use it, the site must have [issue linking](https://confluence.atlassian.com/x/yoXKM) enabled.

- **`jira-cloud-platform-pp-cli issue-link delete`** - Deletes an issue link.

This operation can be accessed anonymously.

**[Permissions](#permissions) required:**

 *  Browse project [project permission](https://confluence.atlassian.com/x/yodKLg) for all the projects containing the issues in the link.
 *  *Link issues* [project permission](https://confluence.atlassian.com/x/yodKLg) for at least one of the projects containing issues in the link.
 *  If [issue-level security](https://confluence.atlassian.com/x/J4lKLg) is configured, permission to view both of the issues.
- **`jira-cloud-platform-pp-cli issue-link get`** - Returns an issue link.

This operation can be accessed anonymously.

**[Permissions](#permissions) required:**

 *  *Browse project* [project permission](https://confluence.atlassian.com/x/yodKLg) for all the projects containing the linked issues.
 *  If [issue-level security](https://confluence.atlassian.com/x/J4lKLg) is configured, permission to view both of the issues.
- **`jira-cloud-platform-pp-cli issue-link link-issues`** - Creates a link between two issues. Use this operation to indicate a relationship between two issues and optionally add a comment to the from (outward) issue. To use this resource the site must have [Issue Linking](https://confluence.atlassian.com/x/yoXKM) enabled.

This resource returns nothing on the creation of an issue link. To obtain the ID of the issue link, use `https://your-domain.atlassian.net/rest/api/3/issue/[linked issue key]?fields=issuelinks`.

If the link request duplicates a link, the response indicates that the issue link was created. If the request included a comment, the comment is added.

This operation can be accessed anonymously.

**[Permissions](#permissions) required:**

 *  *Browse project* [project permission](https://confluence.atlassian.com/x/yodKLg) for all the projects containing the issues to be linked,
 *  *Link issues* [project permission](https://confluence.atlassian.com/x/yodKLg) on the project containing the from (outward) issue,
 *  If [issue-level security](https://confluence.atlassian.com/x/J4lKLg) is configured, issue-level security permission to view the issue.
 *  If the comment has visibility restrictions, belongs to the group or has the role visibility is restricted to.

### issue-link-type

This resource represents [issue link](#api-group-Issue-links) types. Use it to get, create, update, and delete link issue types as well as get lists of all link issue types.

To use it, the site must have [issue linking](https://confluence.atlassian.com/x/yoXKM) enabled.

- **`jira-cloud-platform-pp-cli issue-link-type create`** - Creates an issue link type. Use this operation to create descriptions of the reasons why issues are linked. The issue link type consists of a name and descriptions for a link's inward and outward relationships.

To use this operation, the site must have [issue linking](https://confluence.atlassian.com/x/yoXKM) enabled.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli issue-link-type delete`** - Deletes an issue link type.

To use this operation, the site must have [issue linking](https://confluence.atlassian.com/x/yoXKM) enabled.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli issue-link-type get`** - Returns a list of all issue link types.

To use this operation, the site must have [issue linking](https://confluence.atlassian.com/x/yoXKM) enabled.

This operation can be accessed anonymously.

**[Permissions](#permissions) required:** *Browse projects* [project permission](https://confluence.atlassian.com/x/yodKLg) for a project in the site.
- **`jira-cloud-platform-pp-cli issue-link-type get-issuelinktype`** - Returns an issue link type.

To use this operation, the site must have [issue linking](https://confluence.atlassian.com/x/yoXKM) enabled.

This operation can be accessed anonymously.

**[Permissions](#permissions) required:** *Browse projects* [project permission](https://confluence.atlassian.com/x/yodKLg) for a project in the site.
- **`jira-cloud-platform-pp-cli issue-link-type update`** - Updates an issue link type.

To use this operation, the site must have [issue linking](https://confluence.atlassian.com/x/yoXKM) enabled.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).

### issues

This resource represents Jira issues. Use it to:

 *  create or edit issues, individually or in bulk.
 *  retrieve metadata about the options for creating or editing issues.
 *  delete an issue.
 *  assign a user to an issue.
 *  get issue changelogs.
 *  send notifications about an issue.
 *  get details of the transitions available for an issue.
 *  transition an issue.
 *  Archive issues.
 *  Unarchive issues.
 *  Export archived issues.

- **`jira-cloud-platform-pp-cli issues export-archived`** - Enables admins to retrieve details of all archived issues. Upon a successful request, the admin who submitted it will receive an email with a link to download a CSV file with the issue details.

Note that this API only exports the values of system fields and archival-specific fields (`ArchivedBy` and `ArchivedDate`). Custom fields aren't supported.

**[Permissions](#permissions) required:** Jira admin or site admin: [global permission](https://confluence.atlassian.com/x/x4dKLg)

**License required:** Premium or Enterprise

**Signed-in users only:** This API can't be accessed anonymously.

**Rate limiting:** Only a single request can be active at any given time.

### issuesecurityschemes

Manage issuesecurityschemes

- **`jira-cloud-platform-pp-cli issuesecurityschemes associate-schemes-to-projects`** - Associates an issue security scheme with a project and remaps security levels of issues to the new levels, if provided.

This operation is [asynchronous](#async). Follow the `location` link in the response to determine the status of the task and use [Get task](#api-rest-api-3-task-taskId-get) to obtain subsequent updates.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli issuesecurityschemes create-issue-security-scheme`** - Creates a security scheme with security scheme levels and levels' members. You can create up to 100 security scheme levels and security scheme levels' members per request.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli issuesecurityschemes delete-security-scheme`** - Deletes an issue security scheme.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli issuesecurityschemes get-issue-security-scheme`** - Returns an issue security scheme along with its security levels.

**[Permissions](#permissions) required:**

 *  *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
 *  *Administer Projects* [project permission](https://confluence.atlassian.com/x/yodKLg) for a project that uses the requested issue security scheme.
- **`jira-cloud-platform-pp-cli issuesecurityschemes get-issue-security-schemes`** - Returns all [issue security schemes](https://confluence.atlassian.com/x/J4lKLg).

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli issuesecurityschemes get-security-level-members`** - Returns a [paginated](#pagination) list of issue security level members.

Only issue security level members in the context of classic projects are returned.

Filtering using parameters is inclusive: if you specify both security scheme IDs and level IDs, the result will include all issue security level members from the specified schemes and levels.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli issuesecurityschemes get-security-levels`** - Returns a [paginated](#pagination) list of issue security levels.

Only issue security levels in the context of classic projects are returned.

Filtering using IDs is inclusive: if you specify both security scheme IDs and level IDs, the result will include both specified issue security levels and all issue security levels from the specified schemes.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli issuesecurityschemes search-projects-using-security-schemes`** - Returns a [paginated](#pagination) mapping of projects that are using security schemes. You can provide either one or multiple security scheme IDs or project IDs to filter by. If you don't provide any, this will return a list of all mappings. Only issue security schemes in the context of classic projects are supported. **[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli issuesecurityschemes search-security-schemes`** - Returns a [paginated](#pagination) list of issue security schemes.  
If you specify the project ID parameter, the result will contain issue security schemes and related project IDs you filter by. Use \{@link IssueSecuritySchemeResource\#searchProjectsUsingSecuritySchemes(String, String, Set, Set)\} to obtain all projects related to scheme.

Only issue security schemes in the context of classic projects are returned.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli issuesecurityschemes set-default-levels`** - Sets default issue security levels for schemes.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli issuesecurityschemes update-issue-security-scheme`** - Updates the issue security scheme.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).

### issuetype

Manage issuetype

- **`jira-cloud-platform-pp-cli issuetype create-issue-type`** - Creates an issue type.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli issuetype delete-issue-type`** - Deletes the issue type. If the issue type is in use, all uses are updated with the alternative issue type (`alternativeIssueTypeId`). A list of alternative issue types are obtained from the [Get alternative issue types](#api-rest-api-3-issuetype-id-alternatives-get) resource.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli issuetype get-issue-all-types`** - Returns all issue types.

This operation can be accessed anonymously.

**[Permissions](#permissions) required:** Issue types are only returned as follows:

 *  if the user has the *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg), all issue types are returned.
 *  if the user has the *Browse projects* [project permission](https://confluence.atlassian.com/x/yodKLg) for one or more projects, the issue types associated with the projects the user has permission to browse are returned.
 *  if the user is anonymous then they will be able to access projects with the *Browse projects* for anonymous users
 *  if the user authentication is incorrect they will fall back to anonymous
- **`jira-cloud-platform-pp-cli issuetype get-issue-type`** - Returns an issue type.

This operation can be accessed anonymously.

**[Permissions](#permissions) required:** *Browse projects* [project permission](https://confluence.atlassian.com/x/yodKLg) in a project the issue type is associated with or *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli issuetype get-issue-types-for-project`** - Returns issue types for a project.

This operation can be accessed anonymously.

**[Permissions](#permissions) required:** *Browse projects* [project permission](https://confluence.atlassian.com/x/yodKLg) in the relevant project or *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli issuetype update-issue-type`** - Updates the issue type.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).

### issuetypescheme

Manage issuetypescheme

- **`jira-cloud-platform-pp-cli issuetypescheme assign-issue-type-scheme-to-project`** - Assigns an issue type scheme to a project.

If any issues in the project are assigned issue types not present in the new scheme, the operation will fail. To complete the assignment those issues must be updated to use issue types in the new scheme.

Issue type schemes can only be assigned to classic projects.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli issuetypescheme create-issue-type-scheme`** - Creates an issue type scheme.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli issuetypescheme delete-issue-type-scheme`** - Deletes an issue type scheme.

Only issue type schemes used in classic projects can be deleted. Only issue type schemes not associated with a project can be deleted

A validation error will be returned if the specified scheme is associated with one or more projects. Use [Get issue type scheme API](https://developer.atlassian.com/cloud/jira/platform/rest/v3/api-group-issue-type-schemes/#api-rest-api-3-issuetypescheme-get) (with the projects expand, and id query parameter) to get a list of projects. Then, use [Assign issue type scheme to project API](https://developer.atlassian.com/cloud/jira/platform/rest/v3/api-group-issue-type-schemes/#api-rest-api-3-issuetypescheme-project-put) to associate all projects to another scheme before deleting.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli issuetypescheme get-all-issue-type-schemes`** - Returns a [paginated](#pagination) list of issue type schemes.

Only issue type schemes used in classic projects are returned.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli issuetypescheme get-issue-type-scheme-for-projects`** - Returns a [paginated](#pagination) list of issue type schemes and, for each issue type scheme, a list of the projects that use it.

Only issue type schemes used in classic projects are returned.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli issuetypescheme get-issue-type-schemes-mapping`** - Returns a [paginated](#pagination) list of issue type scheme items.

Only issue type scheme items used in classic projects are returned.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli issuetypescheme update-issue-type-scheme`** - Updates an issue type scheme.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).

### issuetypescreenscheme

Manage issuetypescreenscheme

- **`jira-cloud-platform-pp-cli issuetypescreenscheme assign-issue-type-screen-scheme-to-project`** - Assigns an issue type screen scheme to a project.

Issue type screen schemes can only be assigned to classic projects.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli issuetypescreenscheme create-issue-type-screen-scheme`** - Creates an issue type screen scheme.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli issuetypescreenscheme delete-issue-type-screen-scheme`** - Deletes an issue type screen scheme.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli issuetypescreenscheme get-issue-type-screen-scheme-mappings`** - Returns a [paginated](#pagination) list of issue type screen scheme items.

Only issue type screen schemes used in classic projects are returned.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli issuetypescreenscheme get-issue-type-screen-scheme-project-associations`** - Returns a [paginated](#pagination) list of issue type screen schemes and, for each issue type screen scheme, a list of the projects that use it.

Only issue type screen schemes used in classic projects are returned.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli issuetypescreenscheme get-issue-type-screen-schemes`** - Returns a [paginated](#pagination) list of issue type screen schemes.

Only issue type screen schemes used in classic projects are returned.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli issuetypescreenscheme update-issue-type-screen-scheme`** - Updates an issue type screen scheme.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).

### jira-cloud-platform-search

Manage jira cloud platform search

- **`jira-cloud-platform-pp-cli jira-cloud-platform-search and-reconsile-issues-using-jql`** - Searches for issues using [JQL](https://confluence.atlassian.com/x/egORLQ). Recent updates might not be immediately visible in the returned search results. If you need [read-after-write](https://developer.atlassian.com/cloud/jira/platform/search-and-reconcile/) consistency, you can utilize the `reconcileIssues` parameter to ensure stronger consistency assurances. This operation can be accessed anonymously.

If the JQL query expression is too large to be encoded as a query parameter, use the [POST](#api-rest-api-3-search-post) version of this resource.

**[Permissions](#permissions) required:** Issues are included in the response where the user has:

 *  *Browse projects* [project permission](https://confluence.atlassian.com/x/yodKLg) for the project containing the issue.
 *  If [issue-level security](https://confluence.atlassian.com/x/J4lKLg) is configured, issue-level security permission to view the issue.
- **`jira-cloud-platform-pp-cli jira-cloud-platform-search and-reconsile-issues-using-jql-post`** - Searches for issues using [JQL](https://confluence.atlassian.com/x/egORLQ). Recent updates might not be immediately visible in the returned search results. If you need [read-after-write](https://developer.atlassian.com/cloud/jira/platform/search-and-reconcile/) consistency, you can utilize the `reconcileIssues` parameter to ensure stronger consistency assurances. This operation can be accessed anonymously.

**[Permissions](#permissions) required:** Issues are included in the response where the user has:

 *  *Browse projects* [project permission](https://confluence.atlassian.com/x/yodKLg) for the project containing the issue.
 *  If [issue-level security](https://confluence.atlassian.com/x/J4lKLg) is configured, issue-level security permission to view the issue.
- **`jira-cloud-platform-pp-cli jira-cloud-platform-search count-issues`** - Provide an estimated count of the issues that match the [JQL](https://confluence.atlassian.com/x/egORLQ). Recent updates might not be immediately visible in the returned output. This endpoint requires JQL to be bounded.

This operation can be accessed anonymously.

**[Permissions](#permissions) required:** Issues are included in the response where the user has:

 *  *Browse projects* [project permission](https://confluence.atlassian.com/x/yodKLg) for the project containing the issue.
 *  If [issue-level security](https://confluence.atlassian.com/x/J4lKLg) is configured, issue-level security permission to view the issue.
- **`jira-cloud-platform-pp-cli jira-cloud-platform-search for-issues-using-jql`** - Endpoint is currently being removed. [More details](https://developer.atlassian.com/changelog/#CHANGE-2046)

Searches for issues using [JQL](https://confluence.atlassian.com/x/egORLQ).

If the JQL query expression is too large to be encoded as a query parameter, use the [POST](#api-rest-api-3-search-post) version of this resource.

This operation can be accessed anonymously.

**[Permissions](#permissions) required:** Issues are included in the response where the user has:

 *  *Browse projects* [project permission](https://confluence.atlassian.com/x/yodKLg) for the project containing the issue.
 *  If [issue-level security](https://confluence.atlassian.com/x/J4lKLg) is configured, issue-level security permission to view the issue.
- **`jira-cloud-platform-pp-cli jira-cloud-platform-search for-issues-using-jql-post`** - Endpoint is currently being removed. [More details](https://developer.atlassian.com/changelog/#CHANGE-2046)

Searches for issues using [JQL](https://confluence.atlassian.com/x/egORLQ).

There is a [GET](#api-rest-api-3-search-get) version of this resource that can be used for smaller JQL query expressions.

This operation can be accessed anonymously.

**[Permissions](#permissions) required:** Issues are included in the response where the user has:

 *  *Browse projects* [project permission](https://confluence.atlassian.com/x/yodKLg) for the project containing the issue.
 *  If [issue-level security](https://confluence.atlassian.com/x/J4lKLg) is configured, issue-level security permission to view the issue.

### jira-cloud-platform-version

Manage jira cloud platform version

- **`jira-cloud-platform-pp-cli jira-cloud-platform-version create`** - Creates a project version.

This operation can be accessed anonymously.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg) or *Administer Projects* [project permission](https://confluence.atlassian.com/x/yodKLg) for the project the version is added to.
- **`jira-cloud-platform-pp-cli jira-cloud-platform-version delete`** - Deletes a project version.

Deprecated, use [ Delete and replace version](#api-rest-api-3-version-id-removeAndSwap-post) that supports swapping version values in custom fields, in addition to the swapping for `fixVersion` and `affectedVersion` provided in this resource.

Alternative versions can be provided to update issues that use the deleted version in `fixVersion` or `affectedVersion`. If alternatives are not provided, occurrences of `fixVersion` and `affectedVersion` that contain the deleted version are cleared.

This operation can be accessed anonymously.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg) or *Administer Projects* [project permission](https://confluence.atlassian.com/x/yodKLg) for the project that contains the version.
- **`jira-cloud-platform-pp-cli jira-cloud-platform-version get`** - Returns a project version.

This operation can be accessed anonymously.

**[Permissions](#permissions) required:** *Browse projects* [project permission](https://confluence.atlassian.com/x/yodKLg) for the project containing the version.
- **`jira-cloud-platform-pp-cli jira-cloud-platform-version update`** - Updates a project version.

This operation can be accessed anonymously.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg) or *Administer Projects* [project permission](https://confluence.atlassian.com/x/yodKLg) for the project that contains the version.

### jira-cloud-platform-workflow

Manage jira cloud platform workflow

- **`jira-cloud-platform-pp-cli jira-cloud-platform-workflow create-transition-property`** - This will be removed on [June 1, 2026](https://developer.atlassian.com/cloud/jira/platform/changelog/#CHANGE-2570); add transition properties using [Bulk update workflows](https://developer.atlassian.com/cloud/jira/platform/rest/v3/api-group-workflows/#api-rest-api-3-workflows-update-post) instead.

Adds a property to a workflow transition. Transition properties are used to change the behavior of a transition. For more information, see [Transition properties](https://confluence.atlassian.com/x/zIhKLg#Advancedworkflowconfiguration-transitionproperties) and [Workflow properties](https://confluence.atlassian.com/x/JYlKLg).

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli jira-cloud-platform-workflow delete-inactive`** - Deletes a workflow.

The workflow cannot be deleted if it is:

 *  an active workflow.
 *  a system workflow.
 *  associated with any workflow scheme.
 *  associated with any draft workflow scheme.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli jira-cloud-platform-workflow delete-transition-property`** - This will be removed on [June 1, 2026](https://developer.atlassian.com/cloud/jira/platform/changelog/#CHANGE-2570); delete transition properties using [Bulk update workflows](https://developer.atlassian.com/cloud/jira/platform/rest/v3/api-group-workflows/#api-rest-api-3-workflows-update-post) instead.

Deletes a property from a workflow transition. Transition properties are used to change the behavior of a transition. For more information, see [Transition properties](https://confluence.atlassian.com/x/zIhKLg#Advancedworkflowconfiguration-transitionproperties) and [Workflow properties](https://confluence.atlassian.com/x/JYlKLg).

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli jira-cloud-platform-workflow delete-transition-rule-configurations`** - Deletes workflow transition rules from one or more workflows. These rule types are supported:

 *  [post functions](https://developer.atlassian.com/cloud/jira/platform/modules/workflow-post-function/)
 *  [conditions](https://developer.atlassian.com/cloud/jira/platform/modules/workflow-condition/)
 *  [validators](https://developer.atlassian.com/cloud/jira/platform/modules/workflow-validator/)

Only rules created by the calling Connect app can be deleted.

**Note:** The `draft` parameter in the request body WorkflowId is deprecated and will be removed from this API on [November 2, 2026](https://developer.atlassian.com/cloud/jira/platform/changelog/#CHANGE-3147).

**[Permissions](#permissions) required:** Only Connect apps can use this operation.
- **`jira-cloud-platform-pp-cli jira-cloud-platform-workflow get-paginated`** - This will be removed on [June 1, 2026](https://developer.atlassian.com/cloud/jira/platform/changelog/#CHANGE-2569); use [Search workflows](#api-rest-api-3-workflows-search-get) instead.

Returns a [paginated](#pagination) list of published classic workflows. When workflow names are specified, details of those workflows are returned. Otherwise, all published classic workflows are returned.

This operation does not return next-gen workflows.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli jira-cloud-platform-workflow get-transition-properties`** - This will be removed on [June 1, 2026](https://developer.atlassian.com/cloud/jira/platform/changelog/#CHANGE-2570); fetch transition properties from [Bulk get workflows](https://developer.atlassian.com/cloud/jira/platform/rest/v3/api-group-workflows/#api-rest-api-3-workflows-post) instead.

Returns the properties on a workflow transition. Transition properties are used to change the behavior of a transition. For more information, see [Transition properties](https://confluence.atlassian.com/x/zIhKLg#Advancedworkflowconfiguration-transitionproperties) and [Workflow properties](https://confluence.atlassian.com/x/JYlKLg).

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli jira-cloud-platform-workflow get-transition-rule-configurations`** - Returns a [paginated](#pagination) list of workflows with transition rules. The workflows can be filtered to return only those containing workflow transition rules:

 *  of one or more transition rule types, such as [workflow post functions](https://developer.atlassian.com/cloud/jira/platform/modules/workflow-post-function/).
 *  matching one or more transition rule keys.

Only workflows containing transition rules created by the calling [Connect](https://developer.atlassian.com/cloud/jira/platform/index/#connect-apps) or [Forge](https://developer.atlassian.com/cloud/jira/platform/index/#forge-apps) app are returned.

Due to server-side optimizations, workflows with an empty list of rules may be returned; these workflows can be ignored.

**[Permissions](#permissions) required:** Only [Connect](https://developer.atlassian.com/cloud/jira/platform/index/#connect-apps) or [Forge](https://developer.atlassian.com/cloud/jira/platform/index/#forge-apps) apps can use this operation.
- **`jira-cloud-platform-pp-cli jira-cloud-platform-workflow list-history`** - Returns a list of workflow history entries for a specified workflow id.

**Note:** Stored workflow data expires after 60 days. Additionally, no data from before the 30th of October 2025 is available.

**[Permissions](#permissions) required:**

 *  *Administer Jira* global permission to access all, including project-scoped, workflows
 *  At least one of the *Administer projects* and *View (read-only) workflow* project permissions to access project-scoped workflows
- **`jira-cloud-platform-pp-cli jira-cloud-platform-workflow read-from-history`** - Returns a workflow and related statuses for a specified workflow id and version number.

**Note:** Stored workflow data expires after 60 days. Additionally, no data from before the 30th of October 2025 is available.

**[Permissions](#permissions) required:**

 *  *Administer Jira* global permission to access all, including project-scoped, workflows
 *  At least one of the *Administer projects* and *View (read-only) workflow* project permissions to access project-scoped workflows
- **`jira-cloud-platform-pp-cli jira-cloud-platform-workflow update-transition-property`** - This will be removed on [June 1, 2026](https://developer.atlassian.com/cloud/jira/platform/changelog/#CHANGE-2570); update transition properties using [Bulk update workflows](https://developer.atlassian.com/cloud/jira/platform/rest/v3/api-group-workflows/#api-rest-api-3-workflows-update-post) instead.

Updates a workflow transition by changing the property value. Trying to update a property that does not exist results in a new property being added to the transition. Transition properties are used to change the behavior of a transition. For more information, see [Transition properties](https://confluence.atlassian.com/x/zIhKLg#Advancedworkflowconfiguration-transitionproperties) and [Workflow properties](https://confluence.atlassian.com/x/JYlKLg).

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli jira-cloud-platform-workflow update-transition-rule-configurations`** - Updates configuration of workflow transition rules. The following rule types are supported:

 *  [post functions](https://developer.atlassian.com/cloud/jira/platform/modules/workflow-post-function/)
 *  [conditions](https://developer.atlassian.com/cloud/jira/platform/modules/workflow-condition/)
 *  [validators](https://developer.atlassian.com/cloud/jira/platform/modules/workflow-validator/)

Only rules created by the calling [Connect](https://developer.atlassian.com/cloud/jira/platform/index/#connect-apps) or [Forge](https://developer.atlassian.com/cloud/jira/platform/index/#forge-apps) app can be updated.

To assist with app migration, this operation can be used to:

 *  Disable a rule.
 *  Add a `tag`. Use this to filter rules in the [Get workflow transition rule configurations](https://developer.atlassian.com/cloud/jira/platform/rest/v3/api-group-workflow-transition-rules/#api-rest-api-3-workflow-rule-config-get).

Rules are enabled if the `disabled` parameter is not provided.

**Note:** The `draft` parameter in the request body WorkflowId is deprecated and will be removed from this API on [November 2, 2026](https://developer.atlassian.com/cloud/jira/platform/changelog/#CHANGE-3147).

**[Permissions](#permissions) required:** Only [Connect](https://developer.atlassian.com/cloud/jira/platform/index/#connect-apps) or [Forge](https://developer.atlassian.com/cloud/jira/platform/index/#forge-apps) apps can use this operation.

### jql

This resource represents JQL search auto-complete details. Use it to obtain JQL search auto-complete data and suggestions for use in programmatic construction of queries or custom query builders. It also provides operations to:

 *  convert one or more JQL queries with user identifiers (username or user key) to equivalent JQL queries with account IDs.
 *  convert readable details in one or more JQL queries to IDs where a user doesn't have permission to view the entity whose details are readable.

- **`jira-cloud-platform-pp-cli jql get-auto-complete`** - Returns reference data for JQL searches. This is a downloadable version of the documentation provided in [Advanced searching - fields reference](https://confluence.atlassian.com/x/gwORLQ) and [Advanced searching - functions reference](https://confluence.atlassian.com/x/hgORLQ), along with a list of JQL-reserved words. Use this information to assist with the programmatic creation of JQL queries or the validation of queries built in a custom query builder.

To filter visible field details by project or collapse non-unique fields by field type then [Get field reference data (POST)](#api-rest-api-3-jql-autocompletedata-post) can be used.

This operation can be accessed anonymously.

**[Permissions](#permissions) required:** None.
- **`jira-cloud-platform-pp-cli jql get-auto-complete-post`** - Returns reference data for JQL searches. This is a downloadable version of the documentation provided in [Advanced searching - fields reference](https://confluence.atlassian.com/x/gwORLQ) and [Advanced searching - functions reference](https://confluence.atlassian.com/x/hgORLQ), along with a list of JQL-reserved words. Use this information to assist with the programmatic creation of JQL queries or the validation of queries built in a custom query builder.

This operation can filter the custom fields returned by project. Invalid project IDs in `projectIds` are ignored. System fields are always returned.

It can also return the collapsed field for custom fields. Collapsed fields enable searches to be performed across all fields with the same name and of the same field type. For example, the collapsed field `Component - Component[Dropdown]` enables dropdown fields `Component - cf[10061]` and `Component - cf[10062]` to be searched simultaneously.

**[Permissions](#permissions) required:** None.
- **`jira-cloud-platform-pp-cli jql get-field-auto-complete-for-query-string`** - Returns the JQL search auto complete suggestions for a field.

Suggestions can be obtained by providing:

 *  `fieldName` to get a list of all values for the field.
 *  `fieldName` and `fieldValue` to get a list of values containing the text in `fieldValue`.
 *  `fieldName` and `predicateName` to get a list of all predicate values for the field.
 *  `fieldName`, `predicateName`, and `predicateValue` to get a list of predicate values containing the text in `predicateValue`.

This operation can be accessed anonymously.

**[Permissions](#permissions) required:** None.
- **`jira-cloud-platform-pp-cli jql get-precomputations`** - Returns the list of a function's precomputations along with information about when they were created, updated, and last used. Each precomputation has a `value` \- the JQL fragment to replace the custom function clause with.

**[Permissions](#permissions) required:** This API is only accessible to apps and apps can only inspect their own functions.

The new `read:app-data:jira` OAuth scope is 100% optional now, and not using it won't break your app. However, we recommend adding it to your app's scope list because we will eventually make it mandatory.
- **`jira-cloud-platform-pp-cli jql get-precomputations-by-id`** - Returns function precomputations by IDs, along with information about when they were created, updated, and last used. Each precomputation has a `value` \- the JQL fragment to replace the custom function clause with.

**[Permissions](#permissions) required:** This API is only accessible to apps and apps can only inspect their own functions.

The new `read:app-data:jira` OAuth scope is 100% optional now, and not using it won't break your app. However, we recommend adding it to your app's scope list because we will eventually make it mandatory.
- **`jira-cloud-platform-pp-cli jql match-issues`** - Checks whether one or more issues would be returned by one or more JQL queries.

**[Permissions](#permissions) required:** None, however, issues are only matched against JQL queries where the user has:

 *  *Browse projects* [project permission](https://confluence.atlassian.com/x/yodKLg) for the project that the issue is in.
 *  If [issue-level security](https://confluence.atlassian.com/x/J4lKLg) is configured, issue-level security permission to view the issue.
- **`jira-cloud-platform-pp-cli jql migrate-queries`** - Converts one or more JQL queries with user identifiers (username or user key) to equivalent JQL queries with account IDs.

You may wish to use this operation if your system stores JQL queries and you want to make them GDPR-compliant. For more information about GDPR-related changes, see the [migration guide](https://developer.atlassian.com/cloud/jira/platform/deprecation-notice-user-privacy-api-migration-guide/).

**[Permissions](#permissions) required:** Permission to access Jira.
- **`jira-cloud-platform-pp-cli jql parse-queries`** - Parses and validates JQL queries.

Validation is performed in context of the current user.

This operation can be accessed anonymously.

**[Permissions](#permissions) required:** None.
- **`jira-cloud-platform-pp-cli jql sanitise-queries`** - Sanitizes one or more JQL queries by converting readable details into IDs where a user doesn't have permission to view the entity.

For example, if the query contains the clause *project = 'Secret project'*, and a user does not have browse permission for the project "Secret project", the sanitized query replaces the clause with *project = 12345"* (where 12345 is the ID of the project). If a user has the required permission, the clause is not sanitized. If the account ID is null, sanitizing is performed for an anonymous user.

Note that sanitization doesn't make the queries GDPR-compliant, because it doesn't remove user identifiers (username or user key). If you need to make queries GDPR-compliant, use [Convert user identifiers to account IDs in JQL queries](https://developer.atlassian.com/cloud/jira/platform/rest/v3/api-group-jql/#api-rest-api-3-jql-sanitize-post).

Before sanitization each JQL query is parsed. The queries are returned in the same order that they were passed.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli jql update-precomputations`** - Update the precomputation value of a function created by a Forge/Connect app.

**[Permissions](#permissions) required:** An API for apps to update their own precomputations.

The new `write:app-data:jira` OAuth scope is 100% optional now, and not using it won't break your app. However, we recommend adding it to your app's scope list because we will eventually make it mandatory.

### label

This resource represents available labels. Use it to get available labels for the global label field.

- **`jira-cloud-platform-pp-cli label get-all`** - Returns a [paginated](#pagination) list of labels.

### license

Manage license

- **`jira-cloud-platform-pp-cli license get-approximate-application-count`** - Returns the total approximate number of user accounts for a single Jira license. Note that this information is cached with a 7-day lifecycle and could be stale at the time of call.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli license get-approximate-count`** - Returns the approximate number of user accounts across all Jira licenses. Note that this information is cached with a 7-day lifecycle and could be stale at the time of call.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).

### mypermissions

Manage mypermissions

- **`jira-cloud-platform-pp-cli mypermissions get-my-permissions`** - Returns a list of permissions indicating which permissions the user has. Details of the user's permissions can be obtained in a global, project, issue or comment context.

The user is reported as having a project permission:

 *  in the global context, if the user has the project permission in any project.
 *  for a project, where the project permission is determined using issue data, if the user meets the permission's criteria for any issue in the project. Otherwise, if the user has the project permission in the project.
 *  for an issue, where a project permission is determined using issue data, if the user has the permission in the issue. Otherwise, if the user has the project permission in the project containing the issue.
 *  for a comment, where the user has both the permission to browse the comment and the project permission for the comment's parent issue. Only the BROWSE\_PROJECTS permission is supported. If a `commentId` is provided whose `permissions` does not equal BROWSE\_PROJECTS, a 400 error will be returned.

This means that users may be shown as having an issue permission (such as EDIT\_ISSUES) in the global context or a project context but may not have the permission for any or all issues. For example, if Reporters have the EDIT\_ISSUES permission a user would be shown as having this permission in the global context or the context of a project, because any user can be a reporter. However, if they are not the user who reported the issue queried they would not have EDIT\_ISSUES permission for that issue.

For [Jira Service Management project permissions](https://support.atlassian.com/jira-cloud-administration/docs/customize-jira-service-management-permissions/), this will be evaluated similarly to a user in the customer portal. For example, if the BROWSE\_PROJECTS permission is granted to Service Project Customer - Portal Access, any users with access to the customer portal will have the BROWSE\_PROJECTS permission.

Global permissions are unaffected by context.

This operation can be accessed anonymously.

**[Permissions](#permissions) required:** None.

### mypreferences

Manage mypreferences

- **`jira-cloud-platform-pp-cli mypreferences get-locale`** - Returns the locale for the user.

If the user has no language preference set (which is the default setting) or this resource is accessed anonymous, the browser locale detected by Jira is returned. Jira detects the browser locale using the *Accept-Language* header in the request. However, if this doesn't match a locale available Jira, the site default locale is returned.

This operation can be accessed anonymously.

**[Permissions](#permissions) required:** None.
- **`jira-cloud-platform-pp-cli mypreferences get-preference`** - Returns the value of a preference of the current user.

Note that these keys are deprecated:

 *  *jira.user.locale* The locale of the user. By default this is not set and the user takes the locale of the instance.
 *  *jira.user.timezone* The time zone of the user. By default this is not set and the user takes the timezone of the instance.

These system preferences keys will be deprecated by 15/07/2024. You can still retrieve these keys, but it will not have any impact on Notification behaviour.

 *  *user.notifications.watcher* Whether the user gets notified when they are watcher.
 *  *user.notifications.assignee* Whether the user gets notified when they are assignee.
 *  *user.notifications.reporter* Whether the user gets notified when they are reporter.
 *  *user.notifications.mentions* Whether the user gets notified when they are mentions.

Use [ Update a user profile](https://developer.atlassian.com/cloud/admin/user-management/rest/#api-users-account-id-manage-profile-patch) from the user management REST API to manage timezone and locale instead.

**[Permissions](#permissions) required:** Permission to access Jira.
- **`jira-cloud-platform-pp-cli mypreferences remove-preference`** - Deletes a preference of the user, which restores the default value of system defined settings.

Note that these keys are deprecated:

 *  *jira.user.locale* The locale of the user. By default, not set. The user takes the instance locale.
 *  *jira.user.timezone* The time zone of the user. By default, not set. The user takes the instance timezone.

Use [ Update a user profile](https://developer.atlassian.com/cloud/admin/user-management/rest/#api-users-account-id-manage-profile-patch) from the user management REST API to manage timezone and locale instead.

**[Permissions](#permissions) required:** Permission to access Jira.
- **`jira-cloud-platform-pp-cli mypreferences set-locale`** - Deprecated, use [ Update a user profile](https://developer.atlassian.com/cloud/admin/user-management/rest/#api-users-account-id-manage-profile-patch) from the user management REST API instead.

Sets the locale of the user. The locale must be one supported by the instance of Jira.

**[Permissions](#permissions) required:** Permission to access Jira.
- **`jira-cloud-platform-pp-cli mypreferences set-preference`** - Creates a preference for the user or updates a preference's value by sending a plain text string. For example, `false`. An arbitrary preference can be created with the value containing up to 255 characters. In addition, the following keys define system preferences that can be set or created:

 *  *user.notifications.mimetype* The mime type used in notifications sent to the user. Defaults to `html`.
 *  *user.default.share.private* Whether new [ filters](https://confluence.atlassian.com/x/eQiiLQ) are set to private. Defaults to `true`.
 *  *user.keyboard.shortcuts.disabled* Whether keyboard shortcuts are disabled. Defaults to `false`.
 *  *user.autowatch.disabled* Whether the user automatically watches issues they create or add a comment to. By default, not set: the user takes the instance autowatch setting.
 *  *user.notifiy.own.changes* Whether the user gets notified of their own changes.

Note that these keys are deprecated:

 *  *jira.user.locale* The locale of the user. By default, not set. The user takes the instance locale.
 *  *jira.user.timezone* The time zone of the user. By default, not set. The user takes the instance timezone.

These system preferences keys will be deprecated by 15/07/2024. You can still use these keys to create arbitrary preferences, but it will not have any impact on Notification behaviour.

 *  *user.notifications.watcher* Whether the user gets notified when they are watcher.
 *  *user.notifications.assignee* Whether the user gets notified when they are assignee.
 *  *user.notifications.reporter* Whether the user gets notified when they are reporter.
 *  *user.notifications.mentions* Whether the user gets notified when they are mentions.

Use [ Update a user profile](https://developer.atlassian.com/cloud/admin/user-management/rest/#api-users-account-id-manage-profile-patch) from the user management REST API to manage timezone and locale instead.

**[Permissions](#permissions) required:** Permission to access Jira.

### myself

This resource represents information about the current user, such as basic details, group membership, application roles, preferences, and locale. Use it to get, create, update, and delete (restore default) values of the user's preferences and locale.

- **`jira-cloud-platform-pp-cli myself get-current-user`** - Returns details for the current user.

**[Permissions](#permissions) required:** Permission to access Jira.

### notificationscheme

Manage notificationscheme

- **`jira-cloud-platform-pp-cli notificationscheme create-notification-scheme`** - Creates a notification scheme with notifications. You can create up to 1000 notifications per request.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli notificationscheme delete-notification-scheme`** - Deletes a notification scheme.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli notificationscheme get-notification-scheme`** - Returns a [notification scheme](https://confluence.atlassian.com/x/8YdKLg), including the list of events and the recipients who will receive notifications for those events.

**[Permissions](#permissions) required:** Permission to access Jira, however, the user must have permission to administer at least one project associated with the notification scheme.
- **`jira-cloud-platform-pp-cli notificationscheme get-notification-scheme-to-project-mappings`** - Returns a [paginated](#pagination) mapping of project that have notification scheme assigned. You can provide either one or multiple notification scheme IDs or project IDs to filter by. If you don't provide any, this will return a list of all mappings. Note that only company-managed (classic) projects are supported. This is because team-managed projects don't have a concept of a default notification scheme. The mappings are ordered by projectId.

**[Permissions](#permissions) required:** Permission to access Jira.
- **`jira-cloud-platform-pp-cli notificationscheme get-notification-schemes`** - Returns a [paginated](#pagination) list of [notification schemes](https://confluence.atlassian.com/x/8YdKLg) ordered by the display name.

*Note that you should allow for events without recipients to appear in responses.*

**[Permissions](#permissions) required:** Permission to access Jira, however, the user must have permission to administer at least one project associated with a notification scheme for it to be returned.
- **`jira-cloud-platform-pp-cli notificationscheme update-notification-scheme`** - Updates a notification scheme.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).

### permissions

This resource represents permissions. Use it to obtain details of all permissions and determine whether the user has certain permissions.

- **`jira-cloud-platform-pp-cli permissions get-all`** - Returns all permissions, including:

 *  global permissions.
 *  project permissions.
 *  global permissions added by plugins.

This operation can be accessed anonymously.

**[Permissions](#permissions) required:** None.
- **`jira-cloud-platform-pp-cli permissions get-bulk`** - Returns:

 *  for a list of global permissions, the global permissions granted to a user.
 *  for a list of project permissions and lists of projects and issues, for each project permission a list of the projects and issues a user can access or manipulate.

If no account ID is provided, the operation returns details for the logged in user.

Note that:

 *  Invalid project and issue IDs are ignored.
 *  A maximum of 1000 projects and 1000 issues can be checked.
 *  Null values in `globalPermissions`, `projectPermissions`, `projectPermissions.projects`, and `projectPermissions.issues` are ignored.
 *  Empty strings in `projectPermissions.permissions` are ignored.

**Deprecation notice:** The required OAuth 2.0 scopes will be updated on June 15, 2024.

 *  **Classic**: `read:jira-work`
 *  **Granular**: `read:permission:jira`

This operation can be accessed anonymously.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg) to check the permissions for other users, otherwise none. However, Connect apps can make a call from the app server to the product to obtain permission details for any user, without admin permission. This Connect app ability doesn't apply to calls made using AP.request() in a browser.
- **`jira-cloud-platform-pp-cli permissions get-permitted-projects`** - Returns all the projects where the user is granted a list of project permissions.

This operation can be accessed anonymously.

**[Permissions](#permissions) required:** None.

### permissionscheme

Manage permissionscheme

- **`jira-cloud-platform-pp-cli permissionscheme create-permission-scheme`** - Creates a new permission scheme. You can create a permission scheme with or without defining a set of permission grants.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli permissionscheme delete-permission-scheme`** - Deletes a permission scheme.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli permissionscheme get-all-permission-schemes`** - Returns all permission schemes.

### About permission schemes and grants ###

A permission scheme is a collection of permission grants. A permission grant consists of a `holder` and a `permission`.

#### Holder object ####

The `holder` object contains information about the user or group being granted the permission. For example, the *Administer projects* permission is granted to a group named *Teams in space administrators*. In this case, the type is `"type": "group"`, and the parameter is the group name, `"parameter": "Teams in space administrators"` and the value is group ID, `"value": "ca85fac0-d974-40ca-a615-7af99c48d24f"`.

The `holder` object is defined by the following properties:

 *  `type` Identifies the user or group (see the list of types below).
 *  `parameter` As a group's name can change, use of `value` is recommended. The value of this property depends on the `type`. For example, if the `type` is a group, then you need to specify the group name.
 *  `value` The value of this property depends on the `type`. If the `type` is a group, then you need to specify the group ID. For other `type` it has the same value as `parameter`

The following `types` are available. The expected values for `parameter` and `value` are given in parentheses (some types may not have a `parameter` or `value`):

 *  `anyone` Grant for anonymous users.
 *  `applicationRole` Grant for users with access to the specified application (application name, application name). See [Update product access settings](https://confluence.atlassian.com/x/3YxjL) for more information.
 *  `assignee` Grant for the user currently assigned to an issue.
 *  `group` Grant for the specified group (`parameter` : group name, `value` : group ID).
 *  `groupCustomField` Grant for a user in the group selected in the specified custom field (`parameter` : custom field ID, `value` : custom field ID).
 *  `projectLead` Grant for a project lead.
 *  `projectRole` Grant for the specified project role (`parameter` :project role ID, `value` : project role ID).
 *  `reporter` Grant for the user who reported the issue.
 *  `sd.customer.portal.only` Jira Service Desk only. Grants customers permission to access the customer portal but not Jira. See [Customizing Jira Service Desk permissions](https://confluence.atlassian.com/x/24dKLg) for more information.
 *  `user` Grant for the specified user (`parameter` : user ID - historically this was the userkey but that is deprecated and the account ID should be used, `value` : user ID).
 *  `userCustomField` Grant for a user selected in the specified custom field (`parameter` : custom field ID, `value` : custom field ID).

#### Built-in permissions ####

The [built-in Jira permissions](https://confluence.atlassian.com/x/yodKLg) are listed below. Apps can also define custom permissions. See the [project permission](https://developer.atlassian.com/cloud/jira/platform/modules/project-permission/) and [global permission](https://developer.atlassian.com/cloud/jira/platform/modules/global-permission/) module documentation for more information.

**Administration permissions**

 *  `ADMINISTER_PROJECTS`
 *  `EDIT_WORKFLOW`
 *  `EDIT_ISSUE_LAYOUT`

**Project permissions**

 *  `BROWSE_PROJECTS`
 *  `MANAGE_SPRINTS_PERMISSION` (Jira Software only)
 *  `SERVICEDESK_AGENT` (Jira Service Desk only)
 *  `VIEW_DEV_TOOLS` (Jira Software only)
 *  `VIEW_READONLY_WORKFLOW`

**Issue permissions**

 *  `ASSIGNABLE_USER`
 *  `ASSIGN_ISSUES`
 *  `CLOSE_ISSUES`
 *  `CREATE_ISSUES`
 *  `DELETE_ISSUES`
 *  `EDIT_ISSUES`
 *  `LINK_ISSUES`
 *  `MODIFY_REPORTER`
 *  `MOVE_ISSUES`
 *  `RESOLVE_ISSUES`
 *  `SCHEDULE_ISSUES`
 *  `SET_ISSUE_SECURITY`
 *  `TRANSITION_ISSUES`

**Voters and watchers permissions**

 *  `MANAGE_WATCHERS`
 *  `VIEW_VOTERS_AND_WATCHERS`

**Comments permissions**

 *  `ADD_COMMENTS`
 *  `DELETE_ALL_COMMENTS`
 *  `DELETE_OWN_COMMENTS`
 *  `EDIT_ALL_COMMENTS`
 *  `EDIT_OWN_COMMENTS`

**Attachments permissions**

 *  `CREATE_ATTACHMENTS`
 *  `DELETE_ALL_ATTACHMENTS`
 *  `DELETE_OWN_ATTACHMENTS`

**Time tracking permissions**

 *  `DELETE_ALL_WORKLOGS`
 *  `DELETE_OWN_WORKLOGS`
 *  `EDIT_ALL_WORKLOGS`
 *  `EDIT_OWN_WORKLOGS`
 *  `WORK_ON_ISSUES`

**[Permissions](#permissions) required:** Permission to access Jira.
- **`jira-cloud-platform-pp-cli permissionscheme get-permission-scheme`** - Returns a permission scheme.

**[Permissions](#permissions) required:** Permission to access Jira.
- **`jira-cloud-platform-pp-cli permissionscheme update-permission-scheme`** - Updates a permission scheme. Below are some important things to note when using this resource:

 *  If a permissions list is present in the request, then it is set in the permission scheme, overwriting *all existing* grants.
 *  If you want to update only the name and description, then do not send a permissions list in the request.
 *  Sending an empty list will remove all permission grants from the permission scheme.

If you want to add or delete a permission grant instead of updating the whole list, see [Create permission grant](#api-rest-api-3-permissionscheme-schemeId-permission-post) or [Delete permission scheme entity](#api-rest-api-3-permissionscheme-schemeId-permission-permissionId-delete).

See [About permission schemes and grants](../api-group-permission-schemes/#about-permission-schemes-and-grants) for more details.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).

### plans

This resource represents plans. Use it to get, create, duplicate, update, trash and archive plans.

- **`jira-cloud-platform-pp-cli plans add-atlassian-team`** - Adds an existing Atlassian team to a plan and configures their plannning settings.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli plans archive`** - Archives a plan.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli plans create`** - Creates a plan.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli plans create-only-team`** - Creates a plan-only team and configures their planning settings.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli plans delete-only-team`** - Deletes a plan-only team and their planning settings.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli plans duplicate`** - Duplicates a plan.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli plans get`** - Returns a [paginated](#pagination) list of plans.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli plans get-atlassian-team`** - Returns planning settings for an Atlassian team in a plan.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli plans get-only-team`** - Returns planning settings for a plan-only team.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli plans get-plan`** - Returns a plan.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli plans get-teams`** - Returns a [paginated](#pagination) list of plan-only and Atlassian teams in a plan.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli plans remove-atlassian-team`** - Removes an Atlassian team from a plan and deletes their planning settings.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli plans trash`** - Moves a plan to trash.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli plans update`** - Updates any of the following details of a plan using [JSON Patch](https://datatracker.ietf.org/doc/html/rfc6902).

 *  name
 *  leadAccountId
 *  scheduling
    
     *  estimation with StoryPoints, Days or Hours as possible values
     *  startDate
        
         *  type with DueDate, TargetStartDate, TargetEndDate or DateCustomField as possible values
         *  dateCustomFieldId
     *  endDate
        
         *  type with DueDate, TargetStartDate, TargetEndDate or DateCustomField as possible values
         *  dateCustomFieldId
     *  inferredDates with None, SprintDates or ReleaseDates as possible values
     *  dependencies with Sequential or Concurrent as possible values
 *  issueSources
    
     *  type with Board, Project or Filter as possible values
     *  value
 *  exclusionRules
    
     *  numberOfDaysToShowCompletedIssues
     *  issueIds
     *  workStatusIds
     *  workStatusCategoryIds
     *  issueTypeIds
     *  releaseIds
 *  crossProjectReleases
    
     *  name
     *  releaseIds
 *  customFields
    
     *  customFieldId
     *  filter
 *  permissions
    
     *  type with View or Edit as possible values
     *  holder
        
         *  type with Group or AccountId as possible values
         *  value

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).

*Note that "add" operations do not respect array indexes in target locations. Call the "Get plan" endpoint to find out the order of array elements.*
- **`jira-cloud-platform-pp-cli plans update-atlassian-team`** - Updates any of the following planning settings of an Atlassian team in a plan using [JSON Patch](https://datatracker.ietf.org/doc/html/rfc6902).

 *  planningStyle
 *  issueSourceId
 *  sprintLength
 *  capacity

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).

*Note that "add" operations do not respect array indexes in target locations. Call the "Get Atlassian team in plan" endpoint to find out the order of array elements.*
- **`jira-cloud-platform-pp-cli plans update-only-team`** - Updates any of the following planning settings of a plan-only team using [JSON Patch](https://datatracker.ietf.org/doc/html/rfc6902).

 *  name
 *  planningStyle
 *  issueSourceId
 *  sprintLength
 *  capacity
 *  memberAccountIds

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).

*Note that "add" operations do not respect array indexes in target locations. Call the "Get plan-only team" endpoint to find out the order of array elements.*

### priority

Manage priority

- **`jira-cloud-platform-pp-cli priority create`** - Creates an issue priority.

Deprecation applies to iconUrl param in request body which will be sunset on 16th Mar 2025. For more details refer to [changelog](https://developer.atlassian.com/changelog/#CHANGE-1525).

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli priority delete`** - Deletes an issue priority.

This operation is [asynchronous](#async). Follow the `location` link in the response to determine the status of the task and use [Get task](#api-rest-api-3-task-taskId-get) to obtain subsequent updates.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli priority get`** - Returns an issue priority.

**[Permissions](#permissions) required:** Permission to access Jira.
- **`jira-cloud-platform-pp-cli priority get-priorities`** - Returns the list of all issue priorities.

**[Permissions](#permissions) required:** Permission to access Jira.
- **`jira-cloud-platform-pp-cli priority move-priorities`** - Changes the order of issue priorities.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli priority search-priorities`** - Returns a [paginated](#pagination) list of priorities. The list can contain all priorities or a subset determined by any combination of these criteria:

 *  a list of priority IDs. Any invalid priority IDs are ignored.
 *  a list of project IDs. Only priorities that are available in these projects will be returned. Any invalid project IDs are ignored.
 *  whether the field configuration is a default. This returns priorities from company-managed (classic) projects only, as there is no concept of default priorities in team-managed projects.

**[Permissions](#permissions) required:** Permission to access Jira.
- **`jira-cloud-platform-pp-cli priority set-default`** - Sets default issue priority.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli priority update`** - Updates an issue priority.

At least one request body parameter must be defined.

Deprecation applies to iconUrl param in request body which will be sunset on 16th Mar 2025. For more details refer to [changelog](https://developer.atlassian.com/changelog/#CHANGE-1525).

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).

### priorityscheme

Manage priorityscheme

- **`jira-cloud-platform-pp-cli priorityscheme create-priority-scheme`** - Creates a new priority scheme.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli priorityscheme delete-priority-scheme`** - Deletes a priority scheme.

This operation is only available for priority schemes without any associated projects. Any associated projects must be removed from the priority scheme before this operation can be performed.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli priorityscheme get-available-priorities-by-priority-scheme`** - Returns a [paginated](#pagination) list of priorities available for adding to a priority scheme.

**[Permissions](#permissions) required:** Permission to access Jira.
- **`jira-cloud-platform-pp-cli priorityscheme get-priority-schemes`** - Returns a [paginated](#pagination) list of priority schemes.

**[Permissions](#permissions) required:** Permission to access Jira.
- **`jira-cloud-platform-pp-cli priorityscheme suggested-priorities-for-mappings`** - Returns a [paginated](#pagination) list of priorities that would require mapping, given a change in priorities or projects associated with a priority scheme.

**[Permissions](#permissions) required:** Permission to access Jira.
- **`jira-cloud-platform-pp-cli priorityscheme update-priority-scheme`** - Updates a priority scheme. This includes its details, the lists of priorities and projects in it

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).

### project

This resource represents projects. Use it to get, create, update, and delete projects. Also get statuses available to a project, a project's notification schemes, and update a project's type.

- **`jira-cloud-platform-pp-cli project create`** - Creates a project based on a project type template, as shown in the following table:

| Project Type Key | Project Template Key |  
|--|--|  
| `business` | `com.atlassian.jira-core-project-templates:jira-core-simplified-content-management`, `com.atlassian.jira-core-project-templates:jira-core-simplified-document-approval`, `com.atlassian.jira-core-project-templates:jira-core-simplified-lead-tracking`, `com.atlassian.jira-core-project-templates:jira-core-simplified-process-control`, `com.atlassian.jira-core-project-templates:jira-core-simplified-procurement`, `com.atlassian.jira-core-project-templates:jira-core-simplified-project-management`, `com.atlassian.jira-core-project-templates:jira-core-simplified-recruitment`, `com.atlassian.jira-core-project-templates:jira-core-simplified-task-tracking` |  
| `service_desk` | `com.atlassian.servicedesk:simplified-it-service-management`, `com.atlassian.servicedesk:simplified-external-service-desk`, `com.atlassian.servicedesk:simplified-hr-service-desk`, `com.atlassian.servicedesk:simplified-facilities-service-desk`, `com.atlassian.servicedesk:simplified-legal-service-desk`, `com.atlassian.servicedesk:simplified-analytics-service-desk`, `com.atlassian.servicedesk:simplified-marketing-service-desk`, `com.atlassian.servicedesk:simplified-design-service-desk`, `com.atlassian.servicedesk:simplified-sales-service-desk`, `com.atlassian.servicedesk:simplified-finance-service-desk`, `com.atlassian.servicedesk:company-managed-blank-service-project`, `com.atlassian.servicedesk:company-managed-general-service-project`, `com.atlassian.servicedesk:team-managed-general-service-project`, `com.atlassian.servicedesk:next-gen-it-service-desk`, `com.atlassian.servicedesk:next-gen-hr-service-desk`, `com.atlassian.servicedesk:next-gen-legal-service-desk`, `com.atlassian.servicedesk:next-gen-marketing-service-desk`, `com.atlassian.servicedesk:next-gen-facilities-service-desk`, `com.atlassian.servicedesk:next-gen-analytics-service-desk`, `com.atlassian.servicedesk:next-gen-finance-service-desk`, `com.atlassian.servicedesk:next-gen-design-service-desk`, `com.atlassian.servicedesk:next-gen-sales-service-desk` |  
| `software` | `com.pyxis.greenhopper.jira:gh-simplified-agility-kanban`, `com.pyxis.greenhopper.jira:gh-simplified-agility-scrum`, `com.pyxis.greenhopper.jira:gh-simplified-basic`, `com.pyxis.greenhopper.jira:gh-simplified-kanban-classic`, `com.pyxis.greenhopper.jira:gh-simplified-scrum-classic` |  
| `customer_service` | `com.atlassian.jcs:customer-service-management` |  
The project types are available according to the installed Jira features as follows:

 *  Jira Core, the default, enables `business` projects.
 *  Jira Service Management enables `service_desk` projects.
 *  Jira Software enables `software` projects.

To determine which features are installed, go to **Jira settings** > **Apps** > **Manage apps** and review the System Apps list. To add Jira Software or Jira Service Management into a JIRA instance, use **Jira settings** > **Apps** > **Finding new apps**. For more information, see [ Managing add-ons](https://confluence.atlassian.com/x/S31NLg).

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli project delete`** - Deletes a project.

You can't delete a project if it's archived. To delete an archived project, restore the project and then delete it. To restore a project, use the Jira UI.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli project get`** - Returns the [project details](https://confluence.atlassian.com/x/ahLpNw) for a project.

This operation can be accessed anonymously.

**[Permissions](#permissions) required:** *Browse projects* [project permission](https://confluence.atlassian.com/x/yodKLg) for the project.
- **`jira-cloud-platform-pp-cli project get-accessible-type-by-key`** - Returns a [project type](https://confluence.atlassian.com/x/Var1Nw) if it is accessible to the user.

**[Permissions](#permissions) required:** Permission to access Jira.
- **`jira-cloud-platform-pp-cli project get-all`** - Returns all projects visible to the user. Deprecated, use [ Get projects paginated](#api-rest-api-3-project-search-get) that supports search and pagination.

This operation can be accessed anonymously.

**[Permissions](#permissions) required:** Projects are returned only where the user has *Browse Projects* or *Administer projects* [project permission](https://confluence.atlassian.com/x/yodKLg) for the project.
- **`jira-cloud-platform-pp-cli project get-all-accessible-types`** - Returns all [project types](https://confluence.atlassian.com/x/Var1Nw) with a valid license.
- **`jira-cloud-platform-pp-cli project get-all-types`** - Returns all [project types](https://confluence.atlassian.com/x/Var1Nw), whether or not the instance has a valid license for each type.

This operation can be accessed anonymously.

**[Permissions](#permissions) required:** None.
- **`jira-cloud-platform-pp-cli project get-recent`** - Returns a list of up to 20 projects recently viewed by the user that are still visible to the user.

This operation can be accessed anonymously.

**[Permissions](#permissions) required:** Projects are returned only where the user has one of:

 *  *Browse Projects* [project permission](https://confluence.atlassian.com/x/yodKLg) for the project.
 *  *Administer Projects* [project permission](https://confluence.atlassian.com/x/yodKLg) for the project.
 *  *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli project get-type-by-key`** - Returns a [project type](https://confluence.atlassian.com/x/Var1Nw).

This operation can be accessed anonymously.

**[Permissions](#permissions) required:** None.
- **`jira-cloud-platform-pp-cli project search`** - Returns a [paginated](#pagination) list of projects visible to the user.

This operation can be accessed anonymously.

**[Permissions](#permissions) required:** Projects are returned only where the user has one of:

 *  *Browse Projects* [project permission](https://confluence.atlassian.com/x/yodKLg) for the project.
 *  *Administer Projects* [project permission](https://confluence.atlassian.com/x/yodKLg) for the project.
 *  *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli project update`** - Updates the [project details](https://confluence.atlassian.com/x/ahLpNw) of a project.

All parameters are optional in the body of the request. Schemes will only be updated if they are included in the request, any omitted schemes will be left unchanged.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg). is only needed when changing the schemes or project key. Otherwise you will only need *Administer Projects* [project permission](https://confluence.atlassian.com/x/yodKLg)

### project-category

Manage project category

- **`jira-cloud-platform-pp-cli project-category create`** - Creates a project category.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli project-category get-all-project-categories`** - Returns all project categories.

**[Permissions](#permissions) required:** Permission to access Jira.
- **`jira-cloud-platform-pp-cli project-category get-by-id`** - Returns a project category.

**[Permissions](#permissions) required:** Permission to access Jira.
- **`jira-cloud-platform-pp-cli project-category remove`** - Deletes a project category.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli project-category update`** - Updates a project category.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).

### project-template

This resource represents project templates. Use it to create a new project from a custom template.

- **`jira-cloud-platform-pp-cli project-template create-project-with-custom-template`** - Creates a project based on a custom template provided in the request.

The request body should contain the project details and the capabilities that comprise the project:

 *  `details` \- represents the project details settings
 *  `template` \- represents a list of capabilities responsible for creating specific parts of a project

A capability is defined as a unit of configuration for the project you want to create.

This operation is:

 *  [asynchronous](#async). Follow the `Location` link in the response header to determine the status of the task and use [Get task](#api-rest-api-3-task-taskId-get) to obtain subsequent updates.

***Note: This API is only supported for Jira Enterprise edition.***

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli project-template edit-template`** - Edit custom template

This API endpoint allows you to edit an existing customised template.

***Note: Custom Templates are only supported for Jira Enterprise edition.***
- **`jira-cloud-platform-pp-cli project-template live-template`** - Get custom template

This API endpoint allows you to get a live custom project template details by either templateKey or projectId

***Note: Custom Templates are only supported for Jira Enterprise edition.***
- **`jira-cloud-platform-pp-cli project-template remove-template`** - Remove custom template

This API endpoint allows you to remove a specified customised template

***Note: Custom Templates are only supported for Jira Enterprise edition.***
- **`jira-cloud-platform-pp-cli project-template save-template`** - Save custom template

This API endpoint allows you to save a customised template

***Note: Custom Templates are only supported for Jira Enterprise edition.***

### projects

This resource represents projects. Use it to get, create, update, and delete projects. Also get statuses available to a project, a project's notification schemes, and update a project's type.

- **`jira-cloud-platform-pp-cli projects get-fields`** - Returns a [paginated](#pagination) list of fields for the requested projects and work types.

Only fields that are available for the specified combination of projects and work types are returned. This endpoint allows filtering to specific fields if field IDs are provided.

**[Permissions](#permissions) required:** Permission to access Jira.

### projectvalidate

Manage projectvalidate

- **`jira-cloud-platform-pp-cli projectvalidate get-valid-project-key`** - Validates a project key and, if the key is invalid or in use, generates a valid random string for the project key.

**[Permissions](#permissions) required:** None.
- **`jira-cloud-platform-pp-cli projectvalidate get-valid-project-name`** - Checks that a project name isn't in use. If the name isn't in use, the passed string is returned. If the name is in use, this operation attempts to generate a valid project name based on the one supplied, usually by adding a sequence number. If a valid project name cannot be generated, a 404 response is returned.

**[Permissions](#permissions) required:** None.
- **`jira-cloud-platform-pp-cli projectvalidate validate-project-key`** - Validates a project key by confirming the key is a valid string and not in use.

**[Permissions](#permissions) required:** None.

### redact

Manage redact

- **`jira-cloud-platform-pp-cli redact get-redaction-status`** - Retrieves the current status of a redaction job ID.

The jobStatus will be one of the following:

 *  IN\_PROGRESS - The redaction job is currently in progress
 *  COMPLETED - The redaction job has completed successfully.
 *  PENDING - The redaction job has not started yet
- **`jira-cloud-platform-pp-cli redact redact`** - Submit a job to redact issue field data. This will trigger the redaction of the data in the specified fields asynchronously.

The redaction status can be polled using the job id.

### resolution

Manage resolution

- **`jira-cloud-platform-pp-cli resolution create`** - Creates an issue resolution.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli resolution delete`** - Deletes an issue resolution.

This operation is [asynchronous](#async). Follow the `location` link in the response to determine the status of the task and use [Get task](#api-rest-api-3-task-taskId-get) to obtain subsequent updates.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli resolution get`** - Returns a list of all issue resolution values.

**[Permissions](#permissions) required:** Permission to access Jira.
- **`jira-cloud-platform-pp-cli resolution get-id`** - Returns an issue resolution value.

**[Permissions](#permissions) required:** Permission to access Jira.
- **`jira-cloud-platform-pp-cli resolution move`** - Changes the order of issue resolutions.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli resolution search`** - Returns a [paginated](#pagination) list of resolutions. The list can contain all resolutions or a subset determined by any combination of these criteria:

 *  a list of resolutions IDs.
 *  whether the field configuration is a default. This returns resolutions from company-managed (classic) projects only, as there is no concept of default resolutions in team-managed projects.

**[Permissions](#permissions) required:** Permission to access Jira.
- **`jira-cloud-platform-pp-cli resolution set-default`** - Sets default issue resolution.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli resolution update`** - Updates an issue resolution.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).

### role

Manage role

- **`jira-cloud-platform-pp-cli role create-project`** - Creates a new project role with no [default actors](#api-rest-api-3-resolution-get). You can use the [Add default actors to project role](#api-rest-api-3-role-id-actors-post) operation to add default actors to the project role after creating it.

*Note that although a new project role is available to all projects upon creation, any default actors that are associated with the project role are not added to projects that existed prior to the role being created.*<

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli role delete-project`** - Deletes a project role. You must specify a replacement project role if you wish to delete a project role that is in use.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli role fully-update-project`** - Updates the project role's name and description. You must include both a name and a description in the request.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli role get-all-project`** - Gets a list of all project roles, complete with project role details and default actors.

### About project roles ###

[Project roles](https://support.atlassian.com/jira-cloud-administration/docs/manage-project-roles/) are a flexible way to to associate users and groups with projects. In Jira Cloud, the list of project roles is shared globally with all projects, but each project can have a different set of actors associated with it (unlike groups, which have the same membership throughout all Jira applications).

Project roles are used in [permission schemes](#api-rest-api-3-permissionscheme-get), [email notification schemes](#api-rest-api-3-notificationscheme-get), [issue security levels](#api-rest-api-3-issuesecurityschemes-get), [comment visibility](#api-rest-api-3-comment-list-post), and workflow conditions.

#### Members and actors ####

In the Jira REST API, a member of a project role is called an *actor*. An *actor* is a group or user associated with a project role.

Actors may be set as [default members](https://support.atlassian.com/jira-cloud-administration/docs/manage-project-roles/#Specifying-'default-members'-for-a-project-role) of the project role or set at the project level:

 *  Default actors: Users and groups that are assigned to the project role for all newly created projects. The default actors can be removed at the project level later if desired.
 *  Actors: Users and groups that are associated with a project role for a project, which may differ from the default actors. This enables you to assign a user to different roles in different projects.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli role get-project-by-id`** - Gets the project role details and the default actors associated with the role. The list of default actors is sorted by display name.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli role partial-update-project`** - Updates either the project role's name or its description.

You cannot update both the name and description at the same time using this operation. If you send a request with a name and a description only the name is updated.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).

### screens

This resource represents the screens used to record issue details. Use it to:

 *  get details of all screens.
 *  get details of all the fields available for use on screens.
 *  create screens.
 *  delete screens.
 *  update screens.
 *  add a field to the default screen.

- **`jira-cloud-platform-pp-cli screens add-field-to-default`** - Adds a field to the default tab of the default screen.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli screens create`** - Creates a screen with a default field tab.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli screens delete`** - Deletes a screen. A screen cannot be deleted if it is used in a screen scheme, workflow, or workflow draft.

Only screens used in classic projects can be deleted.
- **`jira-cloud-platform-pp-cli screens get`** - Returns a [paginated](#pagination) list of all screens or those specified by one or more screen IDs.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli screens get-bulk-tabs`** - Returns the list of tabs for a bulk of screens.

**[Permissions](#permissions) required:**

 *  *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli screens update`** - Updates a screen. Only screens used in classic projects can be updated.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).

### screenscheme

Manage screenscheme

- **`jira-cloud-platform-pp-cli screenscheme create-screen-scheme`** - Creates a screen scheme.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli screenscheme delete-screen-scheme`** - Deletes a screen scheme. A screen scheme cannot be deleted if it is used in an issue type screen scheme.

Only screens schemes used in classic projects can be deleted.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli screenscheme get-screen-schemes`** - Returns a [paginated](#pagination) list of screen schemes.

Only screen schemes used in classic projects are returned.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli screenscheme update-screen-scheme`** - Updates a screen scheme. Only screen schemes used in classic projects can be updated.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).

### securitylevel

Manage securitylevel

- **`jira-cloud-platform-pp-cli securitylevel get-issue-security-level`** - Returns details of an issue security level.

Use [Get issue security scheme](#api-rest-api-3-issuesecurityschemes-id-get) to obtain the IDs of issue security levels associated with the issue security scheme.

This operation can be accessed anonymously.

**[Permissions](#permissions) required:** None.

### server-info

This resource provides information about the Jira instance.

- **`jira-cloud-platform-pp-cli server-info get`** - Returns information about the Jira instance.

This operation can be accessed anonymously.

**[Permissions](#permissions) required:** None.

### settings

Manage settings

- **`jira-cloud-platform-pp-cli settings get-issue-navigator-default-columns`** - Returns the default issue navigator columns.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli settings set-issue-navigator-default-columns`** - Sets the default issue navigator columns.

The `columns` parameter accepts a navigable field value and is expressed as HTML form data. To specify multiple columns, pass multiple `columns` parameters. For example, in curl:

`curl -X PUT -d columns=summary -d columns=description https://your-domain.atlassian.net/rest/api/3/settings/columns`

If no column details are sent, then all default columns are removed.

A navigable field is one that can be used as a column on the issue navigator. Find details of navigable issue columns using [Get fields](#api-rest-api-3-field-get).

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).

### status

This resource represents statuses. Use it to search, get, create, delete, and change statuses.

- **`jira-cloud-platform-pp-cli status get`** - Returns a status. The status must be associated with an active workflow to be returned.

If a name is used on more than one status, only the status found first is returned. Therefore, identifying the status by its ID may be preferable.

This operation can be accessed anonymously.

[Permissions](#permissions) required: *Browse projects* [project permission](https://support.atlassian.com/jira-cloud-administration/docs/manage-project-permissions/) for the project.
- **`jira-cloud-platform-pp-cli status get-statuses`** - Returns a list of all statuses associated with active workflows.

This operation can be accessed anonymously.

[Permissions](#permissions) required: *Browse projects* [project permission](https://support.atlassian.com/jira-cloud-administration/docs/manage-project-permissions/) for the project.

### statuscategory

Manage statuscategory

- **`jira-cloud-platform-pp-cli statuscategory get-status-categories`** - Returns a list of all status categories.

**[Permissions](#permissions) required:** Permission to access Jira.
- **`jira-cloud-platform-pp-cli statuscategory get-status-category`** - Returns a status category. Status categories provided a mechanism for categorizing [statuses](#api-rest-api-3-status-idOrName-get).

**[Permissions](#permissions) required:** Permission to access Jira.

### statuses

Manage statuses

- **`jira-cloud-platform-pp-cli statuses create`** - Creates statuses for a global or project scope.

**[Permissions](#permissions) required:**

 *  *Administer projects* [project permission.](https://confluence.atlassian.com/x/yodKLg)
 *  *Administer Jira* [project permission.](https://confluence.atlassian.com/x/yodKLg)
- **`jira-cloud-platform-pp-cli statuses delete-by-id`** - Deletes statuses by ID.

**[Permissions](#permissions) required:**

 *  *Administer projects* [project permission.](https://confluence.atlassian.com/x/yodKLg)
 *  *Administer Jira* [project permission.](https://confluence.atlassian.com/x/yodKLg)
- **`jira-cloud-platform-pp-cli statuses get-by-id`** - Returns a list of the statuses specified by one or more status IDs.

**[Permissions](#permissions) required:**

 *  *Administer projects* [project permission.](https://confluence.atlassian.com/x/yodKLg)
 *  *Administer Jira* [project permission.](https://confluence.atlassian.com/x/yodKLg)
- **`jira-cloud-platform-pp-cli statuses get-by-name`** - Returns a list of the statuses specified by one or more status names.

**[Permissions](#permissions) required:**

 *  *Administer projects* [project permission.](https://confluence.atlassian.com/x/yodKLg)
 *  *Administer Jira* [project permission.](https://confluence.atlassian.com/x/yodKLg)
 *  *Browse projects* [project permission.](https://confluence.atlassian.com/x/yodKLg)
- **`jira-cloud-platform-pp-cli statuses search`** - Returns a [paginated](https://developer.atlassian.com/cloud/jira/platform/rest/v3/intro/#pagination) list of statuses that match a search on name or project.

**[Permissions](#permissions) required:**

 *  *Administer projects* [project permission.](https://confluence.atlassian.com/x/yodKLg)
 *  *Administer Jira* [project permission.](https://confluence.atlassian.com/x/yodKLg)
- **`jira-cloud-platform-pp-cli statuses update`** - Updates statuses by ID.

**[Permissions](#permissions) required:**

 *  *Administer projects* [project permission.](https://confluence.atlassian.com/x/yodKLg)
 *  *Administer Jira* [project permission.](https://confluence.atlassian.com/x/yodKLg)

### task

This resource represents a [long-running asynchronous tasks](#async-operations). Use it to obtain details about the progress of a long-running task or cancel a long-running task.

- **`jira-cloud-platform-pp-cli task get`** - Returns the status of a [long-running asynchronous task](#async).

When a task has finished, this operation returns the JSON blob applicable to the task. See the documentation of the operation that created the task for details. Task details are not permanently retained. As of September 2019, details are retained for 14 days although this period may change without notice.

**Deprecation notice:** The required OAuth 2.0 scopes will be updated on June 15, 2024.

 *  `read:jira-work`

**[Permissions](#permissions) required:** either of:

 *  *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
 *  Creator of the task.

### ui-modifications

Manage ui modifications

- **`jira-cloud-platform-pp-cli ui-modifications create`** - Creates a UI modification. UI modification can only be created by Forge apps.

Each app can define up to 3000 UI modifications. Each UI modification can define up to 1000 contexts. The same context can be assigned to maximum 100 UI modifications.

**Context types:**

 *  **Jira contexts:** For Jira view types, use `projectId` and `issueTypeId`. One field can act as a wildcard. Supported Jira views:
    
     *  `GIC` \- Jira global issue create
     *  `IssueView` \- Jira issue view
     *  `IssueTransition` \- Jira issue transition
 *  **Jira Service Management contexts:** For Jira Service Management view types, use `portalId` and `requestTypeId`. Wildcards are not supported. Supported JSM views:
    
     *  `JSMRequestCreate` \- Jira Service Management request create portal view

**[Permissions](#permissions) required:**

 *  *None* if the UI modification is created without contexts.
 *  *Browse projects* [project permission](https://confluence.atlassian.com/x/yodKLg) for one or more projects, if the UI modification is created with contexts.

The new `write:app-data:jira` OAuth scope is 100% optional now, and not using it won't break your app. However, we recommend adding it to your app's scope list because we will eventually make it mandatory.
- **`jira-cloud-platform-pp-cli ui-modifications delete`** - Deletes a UI modification. All the contexts that belong to the UI modification are deleted too. UI modification can only be deleted by Forge apps.

**[Permissions](#permissions) required:** None.

The new `write:app-data:jira` OAuth scope is 100% optional now, and not using it won't break your app. However, we recommend adding it to your app's scope list because we will eventually make it mandatory.
- **`jira-cloud-platform-pp-cli ui-modifications get`** - Gets UI modifications. UI modifications can only be retrieved by Forge apps.

**[Permissions](#permissions) required:** None.

The new `read:app-data:jira` OAuth scope is 100% optional now, and not using it won't break your app. However, we recommend adding it to your app's scope list because we will eventually make it mandatory.
- **`jira-cloud-platform-pp-cli ui-modifications update`** - Updates a UI modification. UI modification can only be updated by Forge apps.

Each UI modification can define up to 1000 contexts. The same context can be assigned to maximum 100 UI modifications.

**Context types:**

 *  **Jira contexts:** For Jira view types, use `projectId` and `issueTypeId`. One field can act as a wildcard. Supported Jira views:
    
     *  `GIC` \- Jira global issue create
     *  `IssueView` \- Jira issue view
     *  `IssueTransition` \- Jira issue transition
 *  **Jira Service Management contexts:** For Jira Service Management view types, use `portalId` and `requestTypeId`. Wildcards are not supported. Supported JSM views:
    
     *  `JSMRequestCreate` \- Jira Service Management request create portal view

**[Permissions](#permissions) required:**

 *  *None* if the UI modification is created without contexts.
 *  *Browse projects* [project permission](https://confluence.atlassian.com/x/yodKLg) for one or more projects, if the UI modification is created with contexts.

The new `write:app-data:jira` OAuth scope is 100% optional now, and not using it won't break your app. However, we recommend adding it to your app's scope list because we will eventually make it mandatory.

### universal-avatar

Manage universal avatar

- **`jira-cloud-platform-pp-cli universal-avatar delete-avatar`** - Deletes an avatar from a project, issue type or priority.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli universal-avatar get-avatar-image-by-id`** - Returns a project, issue type or priority avatar image by ID.

This operation can be accessed anonymously.

**[Permissions](#permissions) required:**

 *  For system avatars, none.
 *  For custom project avatars, *Browse projects* [project permission](https://confluence.atlassian.com/x/yodKLg) for the project the avatar belongs to.
 *  For custom issue type avatars, *Browse projects* [project permission](https://confluence.atlassian.com/x/yodKLg) for at least one project the issue type is used in.
 *  For priority avatars, none.
- **`jira-cloud-platform-pp-cli universal-avatar get-avatar-image-by-owner`** - Returns the avatar image for a project, issue type or priority.

This operation can be accessed anonymously.

**[Permissions](#permissions) required:**

 *  For system avatars, none.
 *  For custom project avatars, *Browse projects* [project permission](https://confluence.atlassian.com/x/yodKLg) for the project the avatar belongs to.
 *  For custom issue type avatars, *Browse projects* [project permission](https://confluence.atlassian.com/x/yodKLg) for at least one project the issue type is used in.
 *  For priority avatars, none.
- **`jira-cloud-platform-pp-cli universal-avatar get-avatar-image-by-type`** - Returns the default project, issue type or priority avatar image.

This operation can be accessed anonymously.

**[Permissions](#permissions) required:** None.
- **`jira-cloud-platform-pp-cli universal-avatar get-avatars`** - Returns the system and custom avatars for a project, issue type or priority.

This operation can be accessed anonymously.

**[Permissions](#permissions) required:**

 *  for custom project avatars, *Browse projects* [project permission](https://confluence.atlassian.com/x/yodKLg) for the project the avatar belongs to.
 *  for custom issue type avatars, *Browse projects* [project permission](https://confluence.atlassian.com/x/yodKLg) for at least one project the issue type is used in.
 *  for system avatars, none.
 *  for priority avatars, none.
- **`jira-cloud-platform-pp-cli universal-avatar store-avatar`** - Loads a custom avatar for a project, issue type or priority.

Specify the avatar's local file location in the body of the request. Also, include the following headers:

 *  `X-Atlassian-Token: no-check` To prevent XSRF protection blocking the request, for more information see [Special Headers](#special-request-headers).
 *  `Content-Type: image/image type` Valid image types are JPEG, GIF, or PNG.

For example:  
`curl --request POST `

`--user <your-email>:<api_token> `

`--header 'X-Atlassian-Token: no-check' `

`--header 'Content-Type: image/< image_type>' `

`--data-binary "<@/path/to/file/with/your/avatar>" `

`--url 'https://your-domain.atlassian.net/rest/api/3/universal_avatar/type/{type}/owner/{entityId}'`

The avatar is cropped to a square. If no crop parameters are specified, the square originates at the top left of the image. The length of the square's sides is set to the smaller of the height or width of the image.

The cropped image is then used to create avatars of 16x16, 24x24, 32x32, and 48x48 in size.

After creating the avatar use:

 *  [Update issue type](#api-rest-api-3-issuetype-id-put) to set it as the issue type's displayed avatar.
 *  [Set project avatar](#api-rest-api-3-project-projectIdOrKey-avatar-put) to set it as the project's displayed avatar.
 *  [Update priority](#api-rest-api-3-priority-id-put) to set it as the priority's displayed avatar.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).

### user

This resource represent users. Use it to:

 *  get, get a list of, create, and delete users.
 *  get, set, and reset a user's default issue table columns.
 *  get a list of the groups the user belongs to.
 *  get a list of user account IDs for a list of usernames or user keys.

- **`jira-cloud-platform-pp-cli user bulk-get`** - Returns a [paginated](#pagination) list of the users specified by one or more account IDs.

**[Permissions](#permissions) required:** Permission to access Jira.
- **`jira-cloud-platform-pp-cli user bulk-get-migration`** - Returns the account IDs for the users specified in the `key` or `username` parameters. Note that multiple `key` or `username` parameters can be specified.

**[Permissions](#permissions) required:** Permission to access Jira.
- **`jira-cloud-platform-pp-cli user create`** - Creates a user. This resource is retained for legacy compatibility. As soon as a more suitable alternative is available this resource will be deprecated.

**Note:** This API does not support Forge apps.

If the user exists and has access to Jira, the operation returns a 201 status. If the user exists but does not have access to Jira, the operation returns a 400 status.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg). The caller has to be an **organization admin**.
- **`jira-cloud-platform-pp-cli user delete-property`** - Deletes a property from a user.

Note: This operation does not access the [user properties](https://confluence.atlassian.com/x/8YxjL) created and maintained in Jira.

**[Permissions](#permissions) required:**

 *  *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg), to delete a property from any user.
 *  Access to Jira, to delete a property from the calling user's record.
- **`jira-cloud-platform-pp-cli user find`** - Returns a list of active users that match the search string and property.

This operation first applies a filter to match the search string and property, and then takes the filtered users in the range defined by `startAt` and `maxResults`, up to the thousandth user. To get all the users who match the search string and property, use [Get all users](#api-rest-api-3-users-search-get) and filter the records in your code.

This operation can be accessed anonymously.

Privacy controls are applied to the response based on the users' preferences. This could mean, for example, that the user's email address is hidden. See the [Profile visibility overview](https://developer.atlassian.com/cloud/jira/platform/profile-visibility/) for more details.

**[Permissions](#permissions) required:** *Browse users and groups* [global permission](https://confluence.atlassian.com/x/x4dKLg). Anonymous calls or calls by users without the required permission return empty search results.
- **`jira-cloud-platform-pp-cli user find-assignable`** - Returns a list of users that can be assigned to an issue. Use this operation to find the list of users who can be assigned to:

 *  a new issue, by providing the `projectKeyOrId`.
 *  an updated issue, by providing the `issueKey` or `issueId`.
 *  to an issue during a transition (workflow action), by providing the `issueKey` or `issueId` and the transition id in `actionDescriptorId`. You can obtain the IDs of an issue's valid transitions using the `transitions` option in the `expand` parameter of [ Get issue](#api-rest-api-3-issue-issueIdOrKey-get).

In all these cases, you can pass an account ID to determine if a user can be assigned to an issue. The user is returned in the response if they can be assigned to the issue or issue transition.

This operation takes the users in the range defined by `startAt` and `maxResults`, up to the thousandth user, and then returns only the users from that range that can be assigned the issue. This means the operation usually returns fewer users than specified in `maxResults`. To get all the users who can be assigned the issue, use [Get all users](#api-rest-api-3-users-search-get) and filter the records in your code.

Privacy controls are applied to the response based on the users' preferences. This could mean, for example, that the user's email address is hidden. See the [Profile visibility overview](https://developer.atlassian.com/cloud/jira/platform/profile-visibility/) for more details.

**[Permissions](#permissions) required:** *Browse users and groups* [global permission](https://confluence.atlassian.com/x/x4dKLg) or *Assign issues* [project permission](https://confluence.atlassian.com/x/yodKLg)
- **`jira-cloud-platform-pp-cli user find-bulk-assignable`** - Returns a list of users who can be assigned issues in one or more projects. The list may be restricted to users whose attributes match a string.

This operation takes the users in the range defined by `startAt` and `maxResults`, up to the thousandth user, and then returns only the users from that range that can be assigned issues in the projects. This means the operation usually returns fewer users than specified in `maxResults`. To get all the users who can be assigned issues in the projects, use [Get all users](#api-rest-api-3-users-search-get) and filter the records in your code.

Privacy controls are applied to the response based on the users' preferences. This could mean, for example, that the user's email address is hidden. See the [Profile visibility overview](https://developer.atlassian.com/cloud/jira/platform/profile-visibility/) for more details.

This operation can be accessed anonymously.

**[Permissions](#permissions) required:** *Browse Projects* [project permission](https://confluence.atlassian.com/x/yodKLg) for each project specified in `projectKeys`.
- **`jira-cloud-platform-pp-cli user find-by-query`** - Finds users with a structured query and returns a [paginated](#pagination) list of user details.

This operation takes the users in the range defined by `startAt` and `maxResults`, up to the thousandth user, and then returns only the users from that range that match the structured query. This means the operation usually returns fewer users than specified in `maxResults`. To get all the users who match the structured query, use [Get all users](#api-rest-api-3-users-search-get) and filter the records in your code.

**[Permissions](#permissions) required:** *Browse users and groups* [global permission](https://confluence.atlassian.com/x/x4dKLg).

The query statements are:

 *  `is assignee of PROJ` Returns the users that are assignees of at least one issue in project *PROJ*.
 *  `is assignee of (PROJ-1, PROJ-2)` Returns users that are assignees on the issues *PROJ-1* or *PROJ-2*.
 *  `is reporter of (PROJ-1, PROJ-2)` Returns users that are reporters on the issues *PROJ-1* or *PROJ-2*.
 *  `is watcher of (PROJ-1, PROJ-2)` Returns users that are watchers on the issues *PROJ-1* or *PROJ-2*.
 *  `is voter of (PROJ-1, PROJ-2)` Returns users that are voters on the issues *PROJ-1* or *PROJ-2*.
 *  `is commenter of (PROJ-1, PROJ-2)` Returns users that have posted a comment on the issues *PROJ-1* or *PROJ-2*.
 *  `is transitioner of (PROJ-1, PROJ-2)` Returns users that have performed a transition on issues *PROJ-1* or *PROJ-2*.
 *  `[propertyKey].entity.property.path is "property value"` Returns users with the entity property value. For example, if user property `location` is set to value `{"office": {"country": "AU", "city": "Sydney"}}`, then it's possible to use `[location].office.city is "Sydney"` to match the user.

The list of issues can be extended as needed, as in *(PROJ-1, PROJ-2, ... PROJ-n)*. Statements can be combined using the `AND` and `OR` operators to form more complex queries. For example:

`is assignee of PROJ AND [propertyKey].entity.property.path is "property value"`
- **`jira-cloud-platform-pp-cli user find-for-picker`** - Returns a list of users whose attributes match the query term. The returned object includes the `html` field where the matched query term is highlighted with the HTML strong tag. A list of account IDs can be provided to exclude users from the results.

This operation takes the users in the range defined by `maxResults`, up to the thousandth user, and then returns only the users from that range that match the query term. This means the operation usually returns fewer users than specified in `maxResults`. To get all the users who match the query term, use [Get all users](#api-rest-api-3-users-search-get) and filter the records in your code.

Privacy controls are applied to the response based on the users' preferences. This could mean, for example, that the user's email address is hidden. See the [Profile visibility overview](https://developer.atlassian.com/cloud/jira/platform/profile-visibility/) for more details.

This operation can be accessed anonymously.

**[Permissions](#permissions) required:** *Browse users and groups* [global permission](https://confluence.atlassian.com/x/x4dKLg). Anonymous calls and calls by users without the required permission return search results for an exact name match only.
- **`jira-cloud-platform-pp-cli user find-keys-by-query`** - Finds users with a structured query and returns a [paginated](#pagination) list of user keys.

This operation takes the users in the range defined by `startAt` and `maxResults`, up to the thousandth user, and then returns only the users from that range that match the structured query. This means the operation usually returns fewer users than specified in `maxResults`. To get all the users who match the structured query, use [Get all users](#api-rest-api-3-users-search-get) and filter the records in your code.

**[Permissions](#permissions) required:** *Browse users and groups* [global permission](https://confluence.atlassian.com/x/x4dKLg).

The query statements are:

 *  `is assignee of PROJ` Returns the users that are assignees of at least one issue in project *PROJ*.
 *  `is assignee of (PROJ-1, PROJ-2)` Returns users that are assignees on the issues *PROJ-1* or *PROJ-2*.
 *  `is reporter of (PROJ-1, PROJ-2)` Returns users that are reporters on the issues *PROJ-1* or *PROJ-2*.
 *  `is watcher of (PROJ-1, PROJ-2)` Returns users that are watchers on the issues *PROJ-1* or *PROJ-2*.
 *  `is voter of (PROJ-1, PROJ-2)` Returns users that are voters on the issues *PROJ-1* or *PROJ-2*.
 *  `is commenter of (PROJ-1, PROJ-2)` Returns users that have posted a comment on the issues *PROJ-1* or *PROJ-2*.
 *  `is transitioner of (PROJ-1, PROJ-2)` Returns users that have performed a transition on issues *PROJ-1* or *PROJ-2*.
 *  `[propertyKey].entity.property.path is "property value"` Returns users with the entity property value. For example, if user property `location` is set to value `{"office": {"country": "AU", "city": "Sydney"}}`, then it's possible to use `[location].office.city is "Sydney"` to match the user.

The list of issues can be extended as needed, as in *(PROJ-1, PROJ-2, ... PROJ-n)*. Statements can be combined using the `AND` and `OR` operators to form more complex queries. For example:

`is assignee of PROJ AND [propertyKey].entity.property.path is "property value"`
- **`jira-cloud-platform-pp-cli user find-with-all-permissions`** - Returns a list of users who fulfill these criteria:

 *  their user attributes match a search string.
 *  they have a set of permissions for a project or issue.

If no search string is provided, a list of all users with the permissions is returned.

This operation takes the users in the range defined by `startAt` and `maxResults`, up to the thousandth user, and then returns only the users from that range that match the search string and have permission for the project or issue. This means the operation usually returns fewer users than specified in `maxResults`. To get all the users who match the search string and have permission for the project or issue, use [Get all users](#api-rest-api-3-users-search-get) and filter the records in your code.

Privacy controls are applied to the response based on the users' preferences. This could mean, for example, that the user's email address is hidden. See the [Profile visibility overview](https://developer.atlassian.com/cloud/jira/platform/profile-visibility/) for more details.

This operation can be accessed anonymously.

**[Permissions](#permissions) required:**

 *  *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg), to get users for any project.
 *  *Administer Projects* [project permission](https://confluence.atlassian.com/x/yodKLg) for a project, to get users for that project.
- **`jira-cloud-platform-pp-cli user find-with-browse-permission`** - Returns a list of users who fulfill these criteria:

 *  their user attributes match a search string.
 *  they have permission to browse issues.

Use this resource to find users who can browse:

 *  an issue, by providing the `issueKey`.
 *  any issue in a project, by providing the `projectKey`.

This operation takes the users in the range defined by `startAt` and `maxResults`, up to the thousandth user, and then returns only the users from that range that match the search string and have permission to browse issues. This means the operation usually returns fewer users than specified in `maxResults`. To get all the users who match the search string and have permission to browse issues, use [Get all users](#api-rest-api-3-users-search-get) and filter the records in your code.

Privacy controls are applied to the response based on the users' preferences. This could mean, for example, that the user's email address is hidden. See the [Profile visibility overview](https://developer.atlassian.com/cloud/jira/platform/profile-visibility/) for more details.

This operation can be accessed anonymously.

**[Permissions](#permissions) required:** *Browse users and groups* [global permission](https://confluence.atlassian.com/x/x4dKLg). Anonymous calls and calls by users without the required permission return empty search results.
- **`jira-cloud-platform-pp-cli user get`** - Returns a user.

Privacy controls are applied to the response based on the user's preferences. This could mean, for example, that the user's email address is hidden. See the [Profile visibility overview](https://developer.atlassian.com/cloud/jira/platform/profile-visibility/) for more details.

**[Permissions](#permissions) required:** *Browse users and groups* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli user get-default-columns`** - Returns the default [issue table columns](https://confluence.atlassian.com/x/XYdKLg) for the user. If `accountId` is not passed in the request, the calling user's details are returned.

**[Permissions](#permissions) required:**

 *  *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLgl), to get the column details for any user.
 *  Permission to access Jira, to get the calling user's column details.
- **`jira-cloud-platform-pp-cli user get-email`** - Returns a user's email address regardless of the user's profile visibility settings. For Connect apps, this API is only available to apps approved by Atlassian, according to these [guidelines](https://community.developer.atlassian.com/t/guidelines-for-requesting-access-to-email-address/27603). For Forge apps, this API only supports access via asApp() requests.
- **`jira-cloud-platform-pp-cli user get-email-bulk`** - Returns a user's email address regardless of the user's profile visibility settings. For Connect apps, this API is only available to apps approved by Atlassian, according to these [guidelines](https://community.developer.atlassian.com/t/guidelines-for-requesting-access-to-email-address/27603). For Forge apps, this API only supports access via asApp() requests.
- **`jira-cloud-platform-pp-cli user get-groups`** - Returns the groups to which a user belongs.

**[Permissions](#permissions) required:** *Browse users and groups* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli user get-property`** - Returns the value of a user's property. If no property key is provided [Get user property keys](#api-rest-api-3-user-properties-get) is called.

Note: This operation does not access the [user properties](https://confluence.atlassian.com/x/8YxjL) created and maintained in Jira.

**[Permissions](#permissions) required:**

 *  *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg), to get a property from any user.
 *  Access to Jira, to get a property from the calling user's record.
- **`jira-cloud-platform-pp-cli user get-property-keys`** - Returns the keys of all properties for a user.

Note: This operation does not access the [user properties](https://confluence.atlassian.com/x/8YxjL) created and maintained in Jira.

**[Permissions](#permissions) required:**

 *  *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg), to access the property keys on any user.
 *  Access to Jira, to access the calling user's property keys.
- **`jira-cloud-platform-pp-cli user remove`** - Deletes a user. If the operation completes successfully then the user is removed from Jira's user base. This operation does not delete the user's Atlassian account.

**[Permissions](#permissions) required:** Site administration (that is, membership of the *site-admin* [group](https://confluence.atlassian.com/x/24xjL)).
- **`jira-cloud-platform-pp-cli user reset-columns`** - Resets the default [ issue table columns](https://confluence.atlassian.com/x/XYdKLg) for the user to the system default. If `accountId` is not passed, the calling user's default columns are reset.

**[Permissions](#permissions) required:**

 *  *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg), to set the columns on any user.
 *  Permission to access Jira, to set the calling user's columns.
- **`jira-cloud-platform-pp-cli user set-columns`** - Sets the default [ issue table columns](https://confluence.atlassian.com/x/XYdKLg) for the user. If an account ID is not passed, the calling user's default columns are set. If no column details are sent, then all default columns are removed.

The parameters for this resource are expressed as HTML form data. For example, in curl:

`curl -X PUT -d columns=summary -d columns=description https://your-domain.atlassian.net/rest/api/3/user/columns?accountId=5b10ac8d82e05b22cc7d4ef5'`

**[Permissions](#permissions) required:**

 *  *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg), to set the columns on any user.
 *  Permission to access Jira, to set the calling user's columns.
- **`jira-cloud-platform-pp-cli user set-property`** - Sets the value of a user's property. Use this resource to store custom data against a user.

Note: This operation does not access the [user properties](https://confluence.atlassian.com/x/8YxjL) created and maintained in Jira.

**[Permissions](#permissions) required:**

 *  *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg), to set a property on any user.
 *  Access to Jira, to set a property on the calling user's record.

### users

This resource represent users. Use it to:

 *  get, get a list of, create, and delete users.
 *  get, set, and reset a user's default issue table columns.
 *  get a list of the groups the user belongs to.
 *  get a list of user account IDs for a list of usernames or user keys.

- **`jira-cloud-platform-pp-cli users get-all`** - Returns a list of all users, including active users, inactive users and previously deleted users that have an Atlassian account.

Privacy controls are applied to the response based on the users' preferences. This could mean, for example, that the user's email address is hidden. See the [Profile visibility overview](https://developer.atlassian.com/cloud/jira/platform/profile-visibility/) for more details.

**[Permissions](#permissions) required:** *Browse users and groups* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli users get-all-default`** - Returns a list of all users, including active users, inactive users and previously deleted users that have an Atlassian account.

Privacy controls are applied to the response based on the users' preferences. This could mean, for example, that the user's email address is hidden. See the [Profile visibility overview](https://developer.atlassian.com/cloud/jira/platform/profile-visibility/) for more details.

**[Permissions](#permissions) required:** *Browse users and groups* [global permission](https://confluence.atlassian.com/x/x4dKLg).

### webhook

This resource represents webhooks. Webhooks are calls sent to a URL when an event occurs in Jira for issues specified by a JQL query. Only Connect and OAuth 2.0 apps can register and manage webhooks. For more information, see [Webhooks](https://developer.atlassian.com/cloud/jira/platform/webhooks/#registering-a-webhook-via-the-jira-rest-api-for-connect-apps).

- **`jira-cloud-platform-pp-cli webhook delete-by-id`** - Removes webhooks by ID. Only webhooks registered by the calling app are removed. If webhooks created by other apps are specified, they are ignored.

**[Permissions](#permissions) required:** Only [Connect](https://developer.atlassian.com/cloud/jira/platform/#connect-apps) and [OAuth 2.0](https://developer.atlassian.com/cloud/jira/platform/oauth-2-3lo-apps) apps can use this operation.
- **`jira-cloud-platform-pp-cli webhook get-dynamic-for-app`** - Returns a [paginated](#pagination) list of the webhooks registered by the calling app.

**[Permissions](#permissions) required:** Only [Connect](https://developer.atlassian.com/cloud/jira/platform/#connect-apps) and [OAuth 2.0](https://developer.atlassian.com/cloud/jira/platform/oauth-2-3lo-apps) apps can use this operation.
- **`jira-cloud-platform-pp-cli webhook get-failed`** - Returns webhooks that have recently failed to be delivered to the requesting app after the maximum number of retries.

After 72 hours the failure may no longer be returned by this operation.

The oldest failure is returned first.

This method uses a cursor-based pagination. To request the next page use the failure time of the last webhook on the list as the `failedAfter` value or use the URL provided in `next`.

**[Permissions](#permissions) required:** Only [Connect apps](https://developer.atlassian.com/cloud/jira/platform/index/#connect-apps) can use this operation.
- **`jira-cloud-platform-pp-cli webhook refresh`** - Extends the life of webhook. Webhooks registered through the REST API expire after 30 days. Call this operation to keep them alive.

Unrecognized webhook IDs (those that are not found or belong to other apps) are ignored.

**[Permissions](#permissions) required:** Only [Connect](https://developer.atlassian.com/cloud/jira/platform/#connect-apps) and [OAuth 2.0](https://developer.atlassian.com/cloud/jira/platform/oauth-2-3lo-apps) apps can use this operation.
- **`jira-cloud-platform-pp-cli webhook register-dynamic`** - Registers webhooks.

**NOTE:** for non-public OAuth apps, webhooks are delivered only if there is a match between the app owner and the user who registered a dynamic webhook.

**[Permissions](#permissions) required:** Only [Connect](https://developer.atlassian.com/cloud/jira/platform/#connect-apps) and [OAuth 2.0](https://developer.atlassian.com/cloud/jira/platform/oauth-2-3lo-apps) apps can use this operation.

### workflows

This resource represents workflows. Use it to:

 *  Get workflows
 *  Create workflows
 *  Update workflows
 *  Delete inactive workflows
 *  Get workflow capabilities

- **`jira-cloud-platform-pp-cli workflows capabilities`** - Get the list of workflow capabilities for a specific workflow using either the workflow ID, or the project and issue type ID pair. The response includes the scope of the workflow, defined as global/project-based, and a list of project types that the workflow is scoped to. It also includes all rules organised into their broad categories (conditions, validators, actions, triggers, screens) as well as the source location (Atlassian-provided, Connect, Forge).

**[Permissions](#permissions) required:**

 *  *Administer Jira* project permission to access all, including global-scoped, workflows
 *  *Administer projects* project permissions to access project-scoped workflows

The current list of Atlassian-provided rules:

#### Validators ####

A validator rule that checks if a user has the required permissions to execute the transition in the workflow.

##### Permission validator #####

A validator rule that checks if a user has the required permissions to execute the transition in the workflow.

    {
       "ruleKey": "system:check-permission-validator",
       "parameters": {
         "permissionKey": "ADMINISTER_PROJECTS"
       }
     }

Parameters:

 *  `permissionKey` The permission required to perform the transition. Allowed values: [built-in Jira permissions](https://developer.atlassian.com/cloud/jira/platform/rest/v3/api-group-permission-schemes/#built-in-permissions).

##### Parent or child blocking validator #####

A validator to block the child issue's transition depending on the parent issue's status.

    {
       "ruleKey" : "system:parent-or-child-blocking-validator"
       "parameters" : {
         "blocker" : "PARENT"
         "statusIds" : "1,2,3"
       }
     }

Parameters:

 *  `blocker` currently only supports `PARENT`.
 *  `statusIds` a comma-separated list of status IDs.

##### Previous status validator #####

A validator that checks if an issue has transitioned through specified previous status(es) before allowing the current transition to occur.

    {
       "ruleKey": "system:previous-status-validator",
       "parameters": {
         "previousStatusIds": "10014",
         "mostRecentStatusOnly": "true"
       }
     }

Parameters:

 *  `previousStatusIds` a comma-separated list of status IDs, currently only support one ID.
 *  `mostRecentStatusOnly` when `true` only considers the most recent status for the condition evaluation. Allowed values: `true`, `false`.

##### Validate a field value #####

A validation that ensures a specific field's value meets the defined criteria before allowing an issue to transition in the workflow.

Depending on the rule type, the result will vary:

###### Field required ######

    {
       "ruleKey": "system:validate-field-value",
       "parameters": {
         "ruleType": "fieldRequired",
         "fieldsRequired": "assignee",
         "ignoreContext": "true",
         "errorMessage": "An assignee must be set!"
       }
     }

Parameters:

 *  `fieldsRequired` the ID of the field that is required. For a custom field, it would look like `customfield_123`.
 *  `ignoreContext` controls the impact of context settings on field validation. When set to `true`, the validator doesn't check a required field if its context isn't configured for the current issue. When set to `false`, the validator requires a field even if its context is invalid. Allowed values: `true`, `false`.
 *  `errorMessage` is the error message to display if the user does not provide a value during the transition. A default error message will be shown if you don't provide one (Optional).

###### Field changed ######

    {
       "ruleKey": "system:validate-field-value",
       "parameters": {
         "ruleType": "fieldChanged",
         "groupsExemptFromValidation": "6862ac20-8672-4f68-896d-4854f5efb79e",
         "fieldKey": "versions",
         "errorMessage": "Affect versions must be modified before transition"
       }
     }

Parameters:

 *  `groupsExemptFromValidation` a comma-separated list of group IDs to be exempt from the validation.
 *  `fieldKey` the ID of the field that has changed. For a custom field, it would look like `customfield_123`.
 *  `errorMessage` the error message to display if the user does not provide a value during the transition. A default error message will be shown if you don't provide one (Optional).

###### Field has a single value ######

    {
       "ruleKey": "system:validate-field-value",
       "parameters": {
         "ruleType": "fieldHasSingleValue",
         "fieldKey": "created",
         "excludeSubtasks": "true"
       }
     }

Parameters:

 *  `fieldKey` the ID of the field to validate. For a custom field, it would look like `customfield_123`.
 *  `excludeSubtasks` Option to exclude values copied from sub-tasks. Allowed values: `true`, `false`.

###### Field matches regular expression ######

    {
       "ruleKey": "system:validate-field-value",
       "parameters": {
         "ruleType": "fieldMatchesRegularExpression",
         "regexp": "[0-9]{4}",
         "fieldKey": "description"
       }
     }

Parameters:

 *  `regexp` the regular expression used to validate the field\\u2019s content.
 *  `fieldKey` the ID of the field to validate. For a custom field, it would look like `customfield_123`.

###### Date field comparison ######

    {
       "ruleKey": "system:validate-field-value",
       "parameters": {
         "ruleType": "dateFieldComparison",
         "date1FieldKey": "duedate",
         "date2FieldKey": "customfield_10054",
         "includeTime": "true",
         "conditionSelected": ">="
       }
     }

Parameters:

 *  `date1FieldKey` the ID of the first field to compare. For a custom field, it would look like `customfield_123`.
 *  `date2FieldKey` the ID of the second field to compare. For a custom field, it would look like `customfield_123`.
 *  `includeTime` if `true`, compares both date and time. Allowed values: `true`, `false`.
 *  `conditionSelected` the condition to compare with. Allowed values: `>`, `>=`, `=`, `<=`, `<`, `!=`.

###### Date range comparison ######

    {
       "ruleKey": "system:validate-field-value",
       "parameters": {
         "ruleType": "windowDateComparison",
         "date1FieldKey": "customfield_10009",
         "date2FieldKey": "customfield_10054",
         "numberOfDays": "3"
       }
     }

Parameters:

 *  `date1FieldKey` the ID of the first field to compare. For a custom field, it would look like `customfield_123`.
 *  `date2FieldKey` the ID of the second field to compare. For a custom field, it would look like `customfield_123`.
 *  `numberOfDays` maximum number of days past the reference date (`date2FieldKey`) to pass validation.

This rule is composed by aggregating the following legacy rules:

 *  FieldRequiredValidator
 *  FieldChangedValidator
 *  FieldHasSingleValueValidator
 *  RegexpFieldValidator
 *  DateFieldValidator
 *  WindowsDateValidator

##### Pro forma: Forms attached validator #####

Validates that one or more forms are attached to the issue.

    {
       "ruleKey" : "system:proforma-forms-attached"
       "parameters" : {}
     }

##### Proforma: Forms submitted validator #####

Validates that all forms attached to the issue have been submitted.

    {
       "ruleKey" : "system:proforma-forms-submitted"
       "parameters" : {}
     }

#### Conditions ####

Conditions enable workflow rules that govern whether a transition can execute.

##### Check field value #####

A condition rule evaluates as true if a specific field's value meets the defined criteria. This rule ensures that an issue can only transition to the next step in the workflow if the field's value matches the desired condition.

    {
       "ruleKey": "system:check-field-value",
       "parameters": {
         "fieldId": "description",
         "fieldValue": "[\"Done\"]",
         "comparator": "=",
         "comparisonType": "STRING"
       }
     }

Parameters:

 *  `fieldId` The ID of the field to check the value of. For non-system fields, it will look like `customfield_123`. Note: `fieldId` is used interchangeably with the idea of `fieldKey` here, they refer to the same field.
 *  `fieldValue` the list of values to check against the field\\u2019s value.
 *  `comparator` The comparison logic. Allowed values: `>`, `>=`, `=`, `<=`, `<`, `!=`.
 *  `comparisonType` The type of data being compared. Allowed values: `STRING`, `NUMBER`, `DATE`, `DATE_WITHOUT_TIME`, `OPTIONID`.

##### Restrict issue transition #####

This rule ensures that issue transitions are restricted based on user accounts, roles, group memberships, and permissions, maintaining control over who can transition an issue. This condition evaluates as `true` if any of the following criteria is met.

    {
       "ruleKey": "system:restrict-issue-transition",
       "parameters": {
         "accountIds": "allow-reporter,5e68ac137d64450d01a77fa0",
         "roleIds": "10002,10004",
         "groupIds": "703ff44a-7dc8-4f4b-9aa6-a65bf3574fa4",
         "permissionKeys": "ADMINISTER_PROJECTS",
         "groupCustomFields": "customfield_10028",
         "allowUserCustomFields": "customfield_10072,customfield_10144,customfield_10007",
         "denyUserCustomFields": "customfield_10107"
       }
     }

Parameters:

 *  `accountIds` a comma-separated list of the user account IDs. It also allows generic values like: `allow-assignee`, `allow-reporter`, and `accountIds` Note: This is only supported in team-managed projects
 *  `roleIds` a comma-separated list of role IDs.
 *  `groupIds` a comma-separated list of group IDs.
 *  `permissionKeys` a comma-separated list of permission keys. Allowed values: [built-in Jira permissions](https://developer.atlassian.com/cloud/jira/platform/rest/v3/api-group-permission-schemes/#built-in-permissions).
 *  `groupCustomFields` a comma-separated list of group custom field IDs.
 *  `allowUserCustomFields` a comma-separated list of user custom field IDs to allow for issue transition.
 *  `denyUserCustomFields` a comma-separated list of user custom field IDs to deny for issue transition.

This rule is composed by aggregating the following legacy rules:

 *  AllowOnlyAssignee
 *  AllowOnlyReporter
 *  InAnyProjectRoleCondition
 *  InProjectRoleCondition
 *  UserInAnyGroupCondition
 *  UserInGroupCondition
 *  PermissionCondtion
 *  InGroupCFCondition
 *  UserIsInCustomFieldCondition

##### Previous status condition #####

A condition that evaluates based on an issue's previous status(es) and specific criteria.

    {
       "ruleKey" : "system:previous-status-condition"
       "parameters" : {
         "previousStatusIds" : "10004",
         "not": "true",
         "mostRecentStatusOnly" : "true",
         "includeCurrentStatus": "true",
         "ignoreLoopTransitions": "true"
       }
     }

Parameters:

 *  `previousStatusIds` a comma-separated list of status IDs, current only support one ID.
 *  `not` indicates if the condition should be reversed. When `true` it checks that the issue has not been in the selected statuses. Allowed values: `true`, `false`.
 *  `mostRecentStatusOnly` when true only considers the most recent status for the condition evaluation. Allowed values: `true`, `false`.
 *  `includeCurrentStatus` includes the current status when evaluating if the issue has been through the selected statuses. Allowed values: `true`, `false`.
 *  `ignoreLoopTransitions` ignore loop transitions. Allowed values: `true`, `false`.

##### Parent or child blocking condition #####

A condition to block the parent\\u2019s issue transition depending on the child\\u2019s issue status.

    {
       "ruleKey" : "system:parent-or-child-blocking-condition"
       "parameters" : {
         "blocker" : "CHILD",
         "statusIds" : "1,2,3"
       }
     }

Parameters:

 *  `blocker` currently only supports `CHILD`.
 *  `statusIds` a comma-separated list of status IDs.

##### Separation of duties #####

A condition preventing the user from performing, if the user has already performed a transition on the issue.

    {
       "ruleKey": "system:separation-of-duties",
       "parameters": {
         "fromStatusId": "10161",
         "toStatusId": "10160"
       }
     }

Parameters:

 *  `fromStatusId` represents the status ID from which the issue is transitioning. It ensures that the user performing the current transition has not performed any actions when the issue was in the specified status.
 *  `toStatusId` represents the status ID to which the issue is transitioning. It ensures that the user performing the current transition is not the same user who has previously transitioned the issue.

##### Restrict transitions #####

A condition preventing all users from transitioning the issue can also optionally include APIs as well.

    {
       "ruleKey": "system:restrict-from-all-users",
       "parameters": {
         "restrictMode": "users"
       }
     }

Parameters:

 *  `restrictMode` restricts the issue transition including/excluding APIs. Allowed values: `"users"`, `"usersAndAPI"`.

##### Jira Service Management block until approved #####

Block an issue transition until approval. Note: This is only supported in team-managed projects.

    {
       "ruleKey": "system:jsd-approvals-block-until-approved",
       "parameters": {
         "approvalConfigurationJson": "{"statusExternalUuid...}"
       }
     }

Parameters:

 *  `approvalConfigurationJson` a stringified JSON holding the Jira Service Management approval configuration.

##### Jira Service Management block until rejected #####

Block an issue transition until rejected. Note: This is only supported in team-managed projects.

    {
       "ruleKey": "system:jsd-approvals-block-until-rejected",
       "parameters": {
         "approvalConfigurationJson": "{"statusExternalUuid...}"
       }
     }

Parameters:

 *  `approvalConfigurationJson` a stringified JSON holding the Jira Service Management approval configuration.

##### Block in progress approval #####

Condition to block issue transition if there is pending approval. Note: This is only supported in company-managed projects.

    {
       "ruleKey": "system:block-in-progress-approval",
       "parameters": {}
     }

#### Post functions ####

Post functions carry out any additional processing required after a workflow transition is executed.

##### Change assignee #####

A post function rule that changes the assignee of an issue after a transition.

    {
       "ruleKey": "system:change-assignee",
       "parameters": {
         "type": "to-selected-user",
         "accountId": "example-account-id"
       }
     }

Parameters:

 *  `type` the parameter used to determine the new assignee. Allowed values: `to-selected-user`, `to-unassigned`, `to-current-user`, `to-current-user`, `to-default-user`, `to-default-user`
 *  `accountId` the account ID of the user to assign the issue to. This parameter is required only when the type is `"to-selected-user"`.

##### Copy field value #####

A post function that automates the process of copying values between fields during a specific transition, ensuring data consistency and reducing manual effort.

    {
       "ruleKey": "system:copy-value-from-other-field",
       "parameters": {
         "sourceFieldKey": "description",
         "targetFieldKey": "components",
         "issueSource": "SAME"
       }
     }

Parameters:

 *  `sourceFieldKey` the field key to copy from. For a custom field, it would look like `customfield_123`
 *  `targetFieldKey` the field key to copy to. For a custom field, it would look like `customfield_123`
 *  `issueSource` `SAME` or `PARENT`. Defaults to `SAME` if no value is provided.

##### Update field #####

A post function that updates or appends a specific field with the given value.

    {
       "ruleKey": "system:update-field",
       "parameters": {
         "field": "customfield_10056",
         "value": "asdf",
         "mode": "append"
       }
     }

Parameters:

 *  `field` the ID of the field to update. For a custom field, it would look like `customfield_123`
 *  `value` the value to update the field with.
 *  `mode` `append` or `replace`. Determines if a value will be appended to the current value, or if the current value will be replaced.

##### Trigger webhook #####

A post function that automatically triggers a predefined webhook when a transition occurs in the workflow.

    {
       "ruleKey": "system:trigger-webhook",
       "parameters": {
         "webhookId": "1"
       }
     }

Parameters:

 *  `webhookId` the ID of the webhook.

#### Screen ####

##### Remind people to update fields #####

A screen rule that prompts users to update a specific field when they interact with an issue screen during a transition. This rule is useful for ensuring that users provide or modify necessary information before moving an issue to the next step in the workflow.

    {
       "ruleKey": "system:remind-people-to-update-fields",
       "params": {
         "remindingFieldIds": "assignee,customfield_10025",
         "remindingMessage": "The message",
         "remindingAlwaysAsk": "true"
       }
     }

Parameters:

 *  `remindingFieldIds` a comma-separated list of field IDs. Note: `fieldId` is used interchangeably with the idea of `fieldKey` here, they refer to the same field.
 *  `remindingMessage` the message to display when prompting the users to update the fields.
 *  `remindingAlwaysAsk` always remind to update fields. Allowed values: `true`, `false`.

##### Shared transition screen #####

A common screen that is shared between transitions in a workflow.

    {
       "ruleKey": "system:transition-screen",
       "params": {
         "screenId": "3"
       }
     }

Parameters:

 *  `screenId` the ID of the screen.

#### Connect & Forge ####

##### Connect rules #####

Validator/Condition/Post function for Connect app.

    {
       "ruleKey": "connect:expression-validator",
       "parameters": {
         "appKey": "com.atlassian.app",
         "config": "",
         "id": "90ce590f-e90c-4cd3-8281-165ce41f2ac3",
         "disabled": "false",
         "tag": ""
       }
     }

Parameters:

 *  `ruleKey` Validator: `connect:expression-validator`, Condition: `connect:expression-condition`, and Post function: `connect:remote-workflow-function`
 *  `appKey` the reference to the Connect app
 *  `config` a JSON payload string describing the configuration
 *  `id` the ID of the rule
 *  `disabled` determine if the Connect app is disabled. Allowed values: `true`, `false`.
 *  `tag` additional tags for the Connect app

##### Forge rules #####

Validator/Condition/Post function for Forge app.

    {
       "ruleKey": "forge:expression-validator",
       "parameters": {
         "key": "ari:cloud:ecosystem::extension/{appId}/{environmentId}/static/{moduleKey}",
         "config": "{"searchString":"workflow validator"}",
         "id": "a865ddf6-bb3f-4a7b-9540-c2f8b3f9f6c2",
         "disabled": "false",
         "tag": ""
       }
     }

Parameters:

 *  `ruleKey` Validator: `forge:expression-validator`, Condition: `forge:expression-condition`, and Post function: `forge:workflow-post-function`
 *  `key` the identifier for the Forge app
 *  `config` the persistent stringified JSON configuration for the Forge rule
 *  `id` the ID of the Forge rule
 *  `disabled` determine if the Forge app is disabled. Allowed values: `true`, `false`.
 *  `tag` additional tags for the Forge app
- **`jira-cloud-platform-pp-cli workflows create`** - Create workflows and related statuses.

**[Permissions](#permissions) required:**

 *  *Administer Jira* project permission to create all, including global-scoped, workflows
 *  *Administer projects* project permissions to create project-scoped workflows
- **`jira-cloud-platform-pp-cli workflows get-default-editor`** - Get the user's default workflow editor. This can be either the new editor or the legacy editor.
- **`jira-cloud-platform-pp-cli workflows read`** - Returns a list of workflows and related statuses by providing workflow names, workflow IDs, or project and issue types.

**[Permissions](#permissions) required:**

 *  *Administer Jira* global permission to access all, including project-scoped, workflows
 *  At least one of the *Administer projects* and *View (read-only) workflow* project permissions to access project-scoped workflows
- **`jira-cloud-platform-pp-cli workflows read-previews`** - Returns a requested workflow within a given project. The response provides a read-only preview of the workflow, omitting full configuration details.

**[Permissions](#permissions) required:**

 *  At least one of the *Administer projects* and *View (read-only) workflow* project permissions
- **`jira-cloud-platform-pp-cli workflows search`** - Returns a [paginated](#pagination) list of global and project workflows. If workflow names are specified in the query string, details of those workflows are returned. Otherwise, all workflows are returned.

**[Permissions](#permissions) required:**

 *  *Administer Jira* global permission to access all, including project-scoped, workflows
 *  At least one of the *Administer projects* and *View (read-only) workflow* project permissions to access project-scoped workflows
- **`jira-cloud-platform-pp-cli workflows update`** - Update workflows and related statuses.

**[Permissions](#permissions) required:**

 *  *Administer Jira* project permission to create all, including global-scoped, workflows
 *  *Administer projects* project permissions to create project-scoped workflows
- **`jira-cloud-platform-pp-cli workflows validate-create`** - Validate the payload for bulk create workflows.

**[Permissions](#permissions) required:**

 *  *Administer Jira* project permission to create all, including global-scoped, workflows
 *  *Administer projects* project permissions to create project-scoped workflows
- **`jira-cloud-platform-pp-cli workflows validate-update`** - Validate the payload for bulk update workflows.

**[Permissions](#permissions) required:**

 *  *Administer Jira* project permission to create all, including global-scoped, workflows
 *  *Administer projects* project permissions to create project-scoped workflows

### workflowscheme

Manage workflowscheme

- **`jira-cloud-platform-pp-cli workflowscheme assign-scheme-to-project`** - Assigns a workflow scheme to a project. This operation is performed only when there are no issues in the project.

Workflow schemes can only be assigned to classic projects.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli workflowscheme create-workflow-scheme`** - Creates a workflow scheme.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli workflowscheme delete-workflow-scheme`** - Deletes a workflow scheme. Note that a workflow scheme cannot be deleted if it is active (that is, being used by at least one project).

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli workflowscheme get-all-workflow-schemes`** - Returns a [paginated](#pagination) list of all workflow schemes, not including draft workflow schemes.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli workflowscheme get-required-workflow-scheme-mappings`** - Gets the required status mappings for the desired changes to a workflow scheme. The results are provided per issue type and workflow. When updating a workflow scheme, status mappings can be provided per issue type, per workflow, or both.

**[Permissions](#permissions) required:**

 *  *Administer Jira* permission to update all, including global-scoped, workflow schemes.
 *  *Administer projects* project permission to update project-scoped workflow schemes.
- **`jira-cloud-platform-pp-cli workflowscheme get-workflow-scheme`** - Returns a workflow scheme.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli workflowscheme get-workflow-scheme-project-associations`** - Returns a list of the workflow schemes associated with a list of projects. Each returned workflow scheme includes a list of the requested projects associated with it. Any team-managed or non-existent projects in the request are ignored and no errors are returned.

If the project is associated with the `Default Workflow Scheme` no ID is returned. This is because the way the `Default Workflow Scheme` is stored means it has no ID.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli workflowscheme read-workflow-schemes`** - Returns a list of workflow schemes by providing workflow scheme IDs or project IDs.

**[Permissions](#permissions) required:**

 *  *Administer Jira* global permission to access all, including project-scoped, workflow schemes
 *  *Administer projects* project permissions to access project-scoped workflow schemes
- **`jira-cloud-platform-pp-cli workflowscheme switch-workflow-scheme-for-project`** - Switches a workflow scheme for a project.

Workflow schemes can only be assigned to classic projects.

**Calculating required mappings:** If statuses from the current workflow scheme won't exist in the target workflow scheme, you must provide `mappingsByIssueTypeOverride` to specify how issues with those statuses should be migrated. Use [the required workflow scheme mappings API](#api-rest-api-3-workflowscheme-update-mappings-post) to determine which statuses and issue types require mappings.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).
- **`jira-cloud-platform-pp-cli workflowscheme update-schemes`** - Updates company-managed and team-managed project workflow schemes. This API doesn't have a concept of draft, so any changes made to a workflow scheme are immediately available. When changing the available statuses for issue types, an [asynchronous task](#async) migrates the issues as defined in the provided mappings.

**[Permissions](#permissions) required:**

 *  *Administer Jira* project permission to update all, including global-scoped, workflow schemes.
 *  *Administer projects* project permission to update project-scoped workflow schemes.
- **`jira-cloud-platform-pp-cli workflowscheme update-workflow-scheme`** - Updates a company-manged project workflow scheme, including the name, default workflow, issue type to project mappings, and more. If the workflow scheme is active (that is, being used by at least one project), then a draft workflow scheme is created or updated instead, provided that `updateDraftIfNeeded` is set to `true`.

**[Permissions](#permissions) required:** *Administer Jira* [global permission](https://confluence.atlassian.com/x/x4dKLg).

### worklog

Manage worklog

- **`jira-cloud-platform-pp-cli worklog get-for-ids`** - Returns worklog details for a list of worklog IDs.

The returned list of worklogs is limited to 1000 items.

**[Permissions](#permissions) required:** Permission to access Jira, however, worklogs are only returned where either of the following is true:

 *  the worklog is set as *Viewable by All Users*.
 *  the user is a member of a project role or group with permission to view the worklog.
- **`jira-cloud-platform-pp-cli worklog get-ids-of-deleted-since`** - Returns a list of IDs and delete timestamps for worklogs deleted after a date and time.

This resource is paginated, with a limit of 1000 worklogs per page. Each page lists worklogs from oldest to youngest. If the number of items in the date range exceeds 1000, `until` indicates the timestamp of the youngest item on the page. Also, `nextPage` provides the URL for the next page of worklogs. The `lastPage` parameter is set to true on the last page of worklogs.

This resource does not return worklogs deleted during the minute preceding the request.

**[Permissions](#permissions) required:** Permission to access Jira.
- **`jira-cloud-platform-pp-cli worklog get-ids-of-modified-since`** - Returns a list of IDs and update timestamps for worklogs updated after a date and time.

This resource is paginated, with a limit of 1000 worklogs per page. Each page lists worklogs from oldest to youngest. If the number of items in the date range exceeds 1000, `until` indicates the timestamp of the youngest item on the page. Also, `nextPage` provides the URL for the next page of worklogs. The `lastPage` parameter is set to true on the last page of worklogs.

This resource does not return worklogs updated during the minute preceding the request.

**[Permissions](#permissions) required:** Permission to access Jira, however, worklogs are only returned where either of the following is true:

 *  the worklog is set as *Viewable by All Users*.
 *  the user is a member of a project role or group with permission to view the worklog.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
jira-cloud-platform-pp-cli attachment get mock-value

# JSON for scripting and agents
jira-cloud-platform-pp-cli attachment get mock-value --json

# Filter to specific fields
jira-cloud-platform-pp-cli attachment get mock-value --json --select id,name,status

# Dry run — show the request without sending
jira-cloud-platform-pp-cli attachment get mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
jira-cloud-platform-pp-cli attachment get mock-value --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries and `--ignore-missing` to delete retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-jira-cloud-platform -g
```

Then invoke `/pp-jira-cloud-platform <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
claude mcp add jira-cloud-platform jira-cloud-platform-pp-mcp -e JIRA_CLOUD_PLATFORM_OAUTH2=<your-token>
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/jira-cloud-platform-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `JIRA_CLOUD_PLATFORM_OAUTH2` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "jira-cloud-platform": {
      "command": "jira-cloud-platform-pp-mcp",
      "env": {
        "JIRA_CLOUD_PLATFORM_OAUTH2": "<your-key>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
jira-cloud-platform-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/jira-cloud-platform-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `JIRA_CLOUD_PLATFORM_OAUTH2` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `jira-cloud-platform-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $JIRA_CLOUD_PLATFORM_OAUTH2`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
