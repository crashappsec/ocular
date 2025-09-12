# Controller Design

The controller is a crucial component of our system, responsible for managing the lifecycle of resources and ensuring
that the desired state is maintained. This document outlines the design principles, architecture, and implementation
details of the controller.
It is responsible for both reconciling resources and managing webhooks, which are essential for validating and mutating
resources during their creation or update.

## Resource Reconciliation and Webhooks actions

| Resource   | Reconciler                                                      | Create Admission Webhook                                                                                    | Update Admission Webhook                                                            | Delete Admission Webhook                                             |
|------------|-----------------------------------------------------------------|-------------------------------------------------------------------------------------------------------------|-------------------------------------------------------------------------------------|----------------------------------------------------------------------|
| Downloader | -                                                               | -                                                                                                           | -                                                                                   | Ensure no Pipelines reference the Downloader, if so prevent deletion |
| Uploader   | -                                                               | -                                                                                                           | Ensure new "required" parameters aren't added that referenced profiles dont specify | Ensure no Profiles reference the Uploader, if so prevent deletion    |
| Profile    | -                                                               | -                                                                                                           | -                                                                                   | Ensure no Pipelines reference the Profile, if so prevent deletion    |
| Pipeline   | Create and manage scan and upload job along with upload service | Ensure referenced Downloader and Profile exist. Ensure no conflicts between Profile scanners and Downloader | Same as `Create`                                                                    | -                                                                    |
| Crawler    | -                                                               | -                                                                                                           | Ensure new "required" parameters aren't added that referenced profiles dont specify | Ensure no Searches reference the Crawler, if so prevent deletion     |
| Search     | Create and manage search job                                    | Ensure referenced Crawler and Profile exist.                                                                | Same as `Create`                                                                    | -                                                                    |
| CronSearch | Create, manage and schedule Searches on a cron schedule         | Ensure referenced Crawler and Profile exist.                                                                | Same as `Create`                                                                    | -                                                                    |
