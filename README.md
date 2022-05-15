# Awesome-Task-Exchange-System
Homework for course https://education.borshev.com/architecture
## Architecture
![Data domain model](https://github.com/zvrvdmtr/Awesome-Task-Exchange-System/blob/arch/docs/architecture.png)
## Events
We used Storming Events approach to describe communication between components inside application.
### Auth events
![Auth events](https://github.com/zvrvdmtr/Awesome-Task-Exchange-System/blob/arch/docs/auth_events.png)
### Tracker events
![Tracker events](https://github.com/zvrvdmtr/Awesome-Task-Exchange-System/blob/arch/docs/tracker_events.png)
### Account events
![Account events](https://github.com/zvrvdmtr/Awesome-Task-Exchange-System/blob/arch/docs/account_events.png)
### Analytics events
![Analytics events](https://github.com/zvrvdmtr/Awesome-Task-Exchange-System/blob/arch/docs/analytics_events.png)
## Data model and domain model
![Data domain model](https://github.com/zvrvdmtr/Awesome-Task-Exchange-System/blob/arch/docs/domain_and_data_model.png)
## CUD events
![CUD events](https://github.com/zvrvdmtr/Awesome-Task-Exchange-System/blob/arch/docs/cud_events.png)

## How to migrate if you change DB scheme.
1. Add field `jira_id` to DB
2. Run migrations
3. Add code which interact with field `description` and `jira_id` to all consumers
4. Deploy
5. Add code which interact with field `description` and `jira_id` to all producers
6. Deploy
7. Remove old code which interact only with `description` field
8. Deploy
9. Run script for old records with empty field `jira_id`, to split information from field `description`

## Error handling strategy
1. All messages we have not proceed, routes to dead letter exchange
2. Dead letter exchange routes this messages to dead letter queue
3. Queue has TTL, which describe the time after which message will be redirect to "native" exchange
5. Numbers of retries if not limited