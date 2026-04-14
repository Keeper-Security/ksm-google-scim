# Keeper Security Google Workspace SCIM Sync

## Overview
This document outlines the high-level architecture and configuration for the Keeper Security SCIM synchronization tool. Because Google Workspace does not adequately support Team SCIM provisioning natively, this custom Go application bridges the gap by reading users and groups from Google Workspace and pushing them into Keeper Enterprise.

This deployment is entirely native to Google Cloud Platform (GCP) and utilizes Cloud Run, Cloud Scheduler, and GCP Secret Manager.

## Quick Links
* **GitHub Repository:** [WebFX-Marketing/ksm-google-scim](https://github.com/WebFX-Marketing/ksm-google-scim)
* **GCP Project Dashboard:** [webfx-keeper-scim-push](https://console.cloud.google.com/home/dashboard?project=webfx-keeper-scim-push)
* **Cloud Run Service:** [ksm-google-scim](https://console.cloud.google.com/run/detail/us-central1/ksm-google-scim)
* **Cloud Scheduler Job:** [ksm-google-scim (Hourly Trigger)](https://console.cloud.google.com/cloudscheduler/jobs/edit/us-central1/ksm-google-scim?project=webfx-keeper-scim-push)
* **GCP Secret Manager:** [ksm-google-scim Configuration Secret](https://console.cloud.google.com/security/secret-manager/secret/ksm-google-scim/versions?project=webfx-keeper-scim-push)

## High-Level Architecture
1. **Cloud Scheduler** acts as the cron job. It fires an HTTP POST request every hour to the Cloud Run endpoint.
2. **Cloud Run** receives the request and spins up the Go application (`GcpScimSyncHttp`).
3. **Secret Manager** holds all sensitive configuration (Keeper SCIM token, Google Workspace Admin Service Account credentials). The Cloud Run instance dynamically fetches and parses this payload on startup.
4. **Synchronization** occurs as the application connects to the Google Admin API, resolves users and groups, and pushes updates to the Keeper SCIM API.

## Key Details & Troubleshooting
If the sync stops working, here is where to look based on the error behavior. 

Always start by checking the **[Cloud Run Logs](https://console.cloud.google.com/run/detail/us-central1/ksm-google-scim/logs?project=webfx-keeper-scim-push)** for detailed application output.

### 1. Cloud Scheduler Failing (403 Permission Denied)
If the Cloud Scheduler is failing to trigger the function and returning a `403` status, the issue is almost certainly IAM or OIDC Token configuration:
* **Cloud Run Invoker Role:** The Service Account attached to the Cloud Scheduler job *must* have the `Cloud Run Invoker` role applied to the Cloud Run service.
* **Audience Mismatch:** Cloud Scheduler authenticates using an OIDC token. The **Audience** field in the Cloud Scheduler job must *exactly* match the Cloud Run URL **without any trailing slashes** (e.g., `https://ksm-google-scim-1049983427840.us-central1.run.app`).

### 2. Application Crashing (500 Internal Server Error)
If Cloud Scheduler receives a 500 (or if you see application panics in the Cloud Run logs), it usually indicates a failure to read the configuration.
* **Secret Manager Access:** The Service Account that the Cloud Run instance runs as (usually the default Compute Engine service account) *must* have the **`Secret Manager Secret Accessor`** IAM role to read the configuration payload.
* **Payload Formatting:** The configuration in Secret Manager is stored as a single JSON object. Ensure any updates to the secret maintain valid JSON formatting (especially the nested `googleCredentials` block).

### 3. Modifying the Configuration
To update the Keeper token, target SCIM groups, or Google Workspace admin account:
1. Navigate to the **[GCP Secret](https://console.cloud.google.com/security/secret-manager/secret/ksm-google-scim/versions?project=webfx-keeper-scim-push)**.
2. Create a new version of the secret.
3. Update the JSON payload with the new values. The Cloud Run function pulls `latest` on every execution, so changes take effect on the next scheduled run.

#### SCIM Groups Wildcard Support
The `scimGroups` array in the JSON configuration dictates which Google Workspace groups are synced to Keeper. It supports powerful wildcard matching:
* **Prefix Wildcards:** `"keeper-scim-*"` matches any group whose Display Name begins with "keeper-scim-".
* **Domain/Email Wildcards:** `"*@webfx.com"` matches any group email belonging to the `webfx.com` domain.
* **Exact Matches:** Passing `"IT Team"` or `"it@webfx.com"` will match exactly that display name or email.
You can mix and match these rules in the array (e.g., `["keeper-sso-*", "admins@webfx.com"]`).