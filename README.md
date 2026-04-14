![Keeper Secret Manager Google SCIM Push Header](https://github.com/user-attachments/assets/856e2170-d1ce-4262-a425-869e10fd04fc)

# Keeper Secrets Manager : Google SCIM Push

This repository contains the source code that synchronizes Google Workspace Users/Groups and Keeper Enterprise Users/Teams. This is necessary because Google Workspace does not adequately support Team SCIM provisioning.

## Configuration Providers
This tool supports fetching its required configuration via two methods:
1.  **Keeper Secrets Manager (KSM)**: The default and recommended approach.
2.  **Google Cloud Secret Manager (GCP SM)**: For strictly native GCP environments.

## Step by Step Instructions
Read this document: [Google Workspace User and Group Provisioning with Cloud Function](https://docs.keeper.io/en/sso-connect-cloud/identity-provider-setup/g-suite-keeper/google-workspace-user-and-group-provisioning-with-cloud-function)

> This project replicates the `keeper scim push --source=google` [Commander CLI command](https://docs.keeper.io/en/keeperpam/commander-cli/command-reference/enterprise-management-commands/scim-push-configuration) and shares configuration settings with this command.

### Prerequisites
* Keeper Secret Manager enterprise subscription (if using KSM)
* Google Cloud project with billing enabled

### Option 1: Prepare KSM Application
  * Create KSM application or reuse the existing one
  * Share the SCIM configuration record with this KSM application
  * `Add Device` and make sure method is `Configuration File` Base64 encoding.

### Option 2: Prepare GCP Secret Manager
  * Format a single JSON payload containing the required parameters (derived from what KSM would normally provide):
    ```json
    {
      "scimUrl": "https://keepersecurity.com/api/rest/scim/v2/...",
      "scimToken": "your-scim-token",
      "googleAdminAccount": "admin@yourdomain.com",
      "scimGroups": [
          "keeper-scim-*",        // Matches all groups starting with "keeper-scim-"
          "*@keeper.webfx.com",   // Matches all groups on the keeper.webfx.com domain
          "exact-group-name",     // Matches exact group display name
          "group@webfx.com"       // Matches exact group email address
      ],
      "googleCredentials": {
         // Full Google Service Account JSON content here
      }
    }
    ```
  * Save this JSON as a new secret in GCP Secret Manager.
  * **Crucial:** Ensure the Service Account that executes the Cloud Function (usually the Default Compute Service Account, unless you specify otherwise) has the **`Secret Manager Secret Accessor`** role so it can read this payload at runtime.

### Configuration with `gcloud`
1. Clone this repository locally
2. Copy `.env.yaml.sample` to `.env.yaml`
3. Edit `.env.yaml`
   * **If using KSM:**
     * Set `KSM_CONFIG_BASE64` to the content of the KSM configuration file generated at the previous step
     * Set `KSM_RECORD_UID` to configuration record UID created for Commander's `scim push` command
   * **If using GCP SM:**
     * Set `GCP_SECRET_NAME` to the full path of your secret (e.g., `projects/YOUR_PROJECT_ID/secrets/YOUR_SECRET_NAME/versions/latest`)
4. Create Google Cloud function. Replace `<REGION>` placeholder with the GCP region. 
```shell
gcloud functions deploy <PickUniqueFunctionName> \
--gen2 \
--runtime=go125 \
--max-instances=1 \
--memory=512M \
--env-vars-file .env.yaml \
--region=<REGION> \
--timeout=120s \
--source=. \
--entry-point=GcpScimSyncHttp \
--trigger-http \
--no-allow-unauthenticated
```

### Configuration with `Google Console`
1. Clone this repository locally
2. Create `source.zip` file that contains "*.go" and "go.*" matches
```shell
zip source.zip `find . -name "*.go"`
zip source.zip `find . -name "go.*"`
```
3. Login to Google Console
4. Create a new function
   * **If using KSM:**
     * Set `KSM_CONFIG_BASE64` to the content of the KSM configuration file generated at the previous step
     * Set `KSM_RECORD_UID` to configuration record UID created for Commander's `scim push` command
   * **If using GCP SM:**
     * Set `GCP_SECRET_NAME` to the full path of your secret (e.g., `projects/YOUR_PROJECT_ID/secrets/YOUR_SECRET_NAME/versions/latest`)
5. Click `NEXT`
6. Set "Entry point" to `GcpScimSyncHttp`
7. Upload the source code using `source.zip`. "Destination bucket" can be any.
8. Click `DEPLOY`

### Create Cloud Scheduler with `Google Console`
1. Find the created function and copy function URL to the clipboard

2. Search for `scheduler` and select `Cloud Scheduler`
3. Click `CREATE JOB`. 
   * Name your job and set frequency (e.g., `15 * * * *` means every hour at 15th minute).
   * **Target type**: HTTP
   * **URL**: Paste the function URL you copied in step 1.
   * **HTTP method**: POST
   * **Auth header**: Add OIDC token
   * **Service account**: Select a service account to invoke the function.
   * **Audience**: Paste the function URL again, but **remove any trailing slashes** (e.g., `https://ksm-google-scim-...run.app`).

4. Grant the scheduler access to the SCIM function
   * Go to **Cloud Run** in the Google Console.
   * Click your function name (`ksm-google-scim`).
   * Navigate to the **Security** tab and click **Add Principal**.
   * Add the Service Account you selected in the Cloud Scheduler configuration.
   * Assign the role **`Cloud Run Invoker`** and save.

5. Create Scheduler and check it works by clicking `FORCE RUN`
