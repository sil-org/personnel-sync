# Personnel Sync
This application is intended to provide a fairly easy to use way of synchronizing people from a personnel system 
to some other system. In this application there are data _sources_ and _destinations_. Since _destinations_ have their 
own unique APIs and integration methods each _destination_ is developed individually to implement the _Destination_ 
interface. The runtime for this application is configured using a `config.json` file. An example is provided named 
`config.example.json`, however it only has the `GoogleGroups` destination in it so other supported destinations are 
documented below. 

## REST API HTTP Timeout

The default timeout for HTTP requests to REST APIs is 45 seconds. It is possible to override 
the default value via an environment variable named `HTTP_TIMEOUT_SECONDS`, the value of 
which must be an integer between 1 and 600 inclusive. 

Alternatively, you can override the default value and the environment variable value by adding
an `HttpTimeoutSeconds` entry in the `ExtraJSON` entry of your `Source`/`Destination` config entry.

# Config

## Email Alerts

Event Log events with a level of LOG_ALERT or LOG_EMERG will result in an email
alert sent via AWS SES. Note that the LOG_EMERG level is 0, which is the Go
zero-value. Any new event log created without a Level assigned will default to
LOG_EMERG and could result in email alerts being sent.

The following is an example configuration:

```json
{
  "Alert": {
    "AWSRegion": "us-east-1",
    "CharSet": "UTF-8",
    "ReturnToAddr": "no-reply@example.org",
    "SubjectText": "personnel-sync alert",
    "RecipientEmails": [
      "admin@example.org"
    ],
    "AWSAccessKeyID": "ABCD1234",
    "AWSSecretAccessKey": "abcd1234!@#$"
  }
}
```

Alternatively, AWS credentials can be supplied by the Serverless framework by adding
the following configuration to `serverless.yml`:

```
provider:
  iamRoleStatements:
    - Effect: 'Allow'
      Action:
        - 'ses:SendEmail'
      Resource: "*"
```

Both authentication mechanisms are provided in the `lambda-example` directory,
but only one is needed.

## Sentry

Sentry can be added by adding the Sentry DSN into `Runtime` > `SentryDSN`. This can be obtained from the project settings in Sentry. The `Runtime` > `Environment` can also be set to specify the environment, such as `production`, `staging`, or `development`. Tags can also be added by setting the `Tags` object. For example, the following will add a `sync` tag set to `example` for each Sentry error:

```
    "SentryTags": {
      "sync": "example"
    },
```

## Pagination

### `RestAPI`

The RestAPI adapter supports pagination as both a source and as a destination.

#### Properties:
- Scheme -- if specified, must be "pages" for page based or "items" for item based
- FirstIndex -- index of first item/page to fetch, default is 1
- NumberKey -- query string key for the item index or page number
- PageLimit -- index of last page to request, default is 1000
- PageSize -- number of records to return in a page, default is 100
- PageSizeKey -- number of items per page, default is 100

#### Example config

Following is an example configuration for Pagination. Unrelated parameters have 
been omitted for simplicity.

```json
{
  "Source": {
    "Type": "RestAPI",
    "ExtraJSON": {
      "Pagination": {
        "Scheme": "pages",
        "NumberKey": "page",
        "PageSizeKey": "page-size",
        "FirstIndex": "1",
        "PageLimit": "10000",
        "PageSize": "200"
      }
    }
  }
}
```

## Data Filter

### RestAPI

Data retrieved from the API, be it the source or destination, can be filtered to
remove unwanted data. This can be useful in case the API does not offer filter
capability, or its filtering capability is insufficient.

List one or more filters in the `ExtraJSON` configuration. Each filter condition
is added using "AND" conditional logic; each one further restricts the output
data. If the value of an attribute configured in a filter is empty or null, the
record is not included in the output data.

#### Properties
- Attribute -- The name of the attribute to filter on. Does not need to be listed in the sync attributes.
- Expression -- A text expression for which to search. Uses RE2 regular expression syntax.
- Exclude -- If true, records matching the expression are excluded.
- Nullable -- If false, and Attribute is missing or null, an error will be logged and the sync will fail.

#### Example config

Following is an example configuration for Filter. Unrelated parameters have
been omitted for simplicity.

```json
{
  "Source": {
    "Type": "RestAPI",
    "ExtraJSON": {
      "Filters": [
        {
          "Attribute": "active",
          "Expression": "true"
        },
        {
          "Attribute": "email",
          "Expression": "@example\\.com"
        }
      ]
    }
  }
}
```

## Sources

### REST API
Data sources coming from simple API calls can use the `RestAPI` source. Here are some examples of how to configure it:

#### Basic Authentication
```json
{
  "Source": {
    "Type": "RestAPI",
    "ExtraJSON": {
      "ListMethod": "GET",
      "BaseURL": "https://example.com",
      "ResultsJSONContainer": "Results",
      "AuthType": "basic",
      "Username": "username",
      "Password": "password",
      "CompareAttribute": "email",
      "UserAgent": "personnel-sync"
    }
  },
  "SyncSets": [
    {
      "Name": "Sync from REST API",
      "Source": {
        "Paths": ["/users"]
      },
      "Destination": {
          "DisableAdd": false,
          "DisableUpdate": false,
          "DisableDelete": false
      }
    }
  ]
}
```

#### Bearer Token Authentication
```json
{
  "Source": {
    "Type": "RestAPI",
    "ExtraJSON": {
      "ListMethod": "GET",
      "BaseURL": "https://example.com",
      "ResultsJSONContainer": "Results",
      "AuthType": "bearer",
      "Password": "token",
      "CompareAttribute": "email",
      "UserAgent": "personnel-sync"
    }
  }
}
```
`SyncSets` is configured the same as for basic authentication.

#### Salesforce OAuth Authentication

In Salesforce Setup, choose "App Manager", and add a new app. Tick the "Enable OAuth Settings"
box and enter https://login.salesforce.com/services/oauth2/callback in the Callback URL. Add any
required Scopes, such as "Manage user data via APIs (api)". 

Once the app has been created, from App Manager, choose View from the context menu of the new app.
Copy the Consumer Key and paste it in the config.json `Source.ExtraJSON.ClientID` and copy the 
Consumer Secret and paste it in the Client Secret json property.

If you don't already have a Security Token, go to User Settings, My Personal Information, 
Reset My Security Token. Add your username in the config.json Username property, and your password
concatenated with your Security Token in the Password property.

If using a Sandbox org, change the config.json BaseURL property to https://test.salesforce.com/services/oauth2/token

```json
{
  "Source": {
    "Type": "RestAPI",
    "ExtraJSON": {
      "ListMethod": "GET",
      "BaseURL": "https://login.salesforce.com/services/oauth2/token",
      "ResultsJSONContainer": "records",
      "AuthType": "SalesforceOauth",
      "Username": "admin@example.com",
      "Password": "abc123def.ghiJKL",
      "ClientID": "ABCD1234abcd56789_ABCD1234abcd5678ABCD1234abcd5678ABCD1234abcd5678ABCD1.234abcd5678ABC",
      "ClientSecret": "0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF",
      "CompareAttribute": "Email",
      "UserAgent": "personnel-sync"
    }
  },
  "SyncSets": [
    {
      "Name": "Sync from Salesforce to Xyz API",
      "Source": {
        "Paths": ["/services/data/v20.0/query/?q=SELECT%20Email,FirstName,LastName%20FROM%20Contact"]
      },
      "Destination": {
          "DisableAdd": false,
          "DisableUpdate": false,
          "DisableDelete": false
      }
    }
  ]
}
```

`SyncSets` is configured the same as for basic authentication.

### Google Sheets
The Google Sheets source reads records in rows from a Sheets document, where 
the first row contains field names.

If not specified in the configuration, the sheet name is "Sheet1"

Example config:
```json
{
  "Source": {
    "Type": "GoogleSheets",
    "ExtraJSON": {
      "DelegatedAdminEmail": "admin@example.com",
      "GoogleAuth": {
        "type": "service_account",
        "project_id": "abc-theme-123456",
        "private_key_id": "abc123",
        "private_key": "-----BEGIN PRIVATE KEY-----\nMIIabc...\nabc...\n...xyz\n-----END PRIVATE KEY-----\n",
        "client_email": "my-sync-bot@abc-theme-123456.iam.gserviceaccount.com",
        "client_id": "123456789012345678901",
        "auth_uri": "https://accounts.google.com/o/oauth2/auth",
        "token_uri": "https://oauth2.googleapis.com/token",
        "auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",
        "client_x509_cert_url": "https://www.googleapis.com/robot/v1/metadata/x509/my-sync-bot%40abc-theme-123456.iam.gserviceaccount.com"
      }            
    }
  },
  "AttributeMap": [
    {
      "Source": "email",
      "Destination": "email"
    },
    {
      "Source": "employee_id",
      "Destination": "employee_id"
    }
  ],
  "SyncSets": [
    {
      "Name": "Sync from Google Sheets to Xyz API",
      "Source": {
        "SheetID": "putAnActualSheetIDHerejD70xAjqPnOCHlDK3YomH",
        "SheetName": "Sheet2",
        "CompareAttribute": "employee_id"
      },
      "Destination": {
          "DisableAdd": false,
          "DisableUpdate": false,
          "DisableDelete": false
      }
    }
  ]
}
```

## Destinations

### REST API
Destinations conforming to a simple REST API can use the `RestAPI` destination.
Authentication is the same as for a REST API source, except that Salesforce
OAuth is not supported.

Here are some examples of how to configure it:

#### Basic Authentication
```json
{
  "Destination": {
    "Type": "RestAPI",
    "ExtraJSON": {
      "ListMethod": "GET",
      "CreateMethod": "POST",
      "BaseURL": "https://example.com",
      "ResultsJSONContainer": "Results",
      "AuthType": "basic",
      "Username": "username",
      "Password": "password",
      "CompareAttribute": "email",
      "UserAgent": "personnel-sync"
    }
  }
}
```

#### Bearer Token Authentication
```json
{
  "Destination": {
    "Type": "RestAPI",
    "ExtraJSON": {
      "ListMethod": "GET",
      "CreateMethod": "POST",
      "UpdateMethod": "PUT",
      "DeleteMethod": "DELETE",
      "IDAttribute": "id",
      "BaseURL": "https://example.com",
      "ResultsJSONContainer": "Results",
      "AuthType": "bearer",
      "Password": "token",
      "CompareAttribute": "email",
      "UserAgent": "personnel-sync"
    }
  },
  "SyncSets": [
    {
      "Name": "Sync from personnel to REST API",
      "Source": {
          "Paths": ["/user-report"]
      },
      "Destination": {
        "Paths": ["/users"],
        "CreatePath": "/users",
        "UpdatePath": "/users/{id}",
        "DeletePath": "/users/{id}"
      }
    }
  ]
}
```

### Google Contacts
This destination can create, update, and delete Contact records in the Google
Shared Contacts list.

The compare attribute is `email`. A limited subset of contact properties are
available to be updated. __WARNING:__ On update, all properties are modified even
if absent from the configuration. Omitted properties are set to empty. One
exception is `fullName` which is filled in by Google with 
`givenName` + `familyName`

| property       | Google property                |
|----------------|--------------------------------|
| id             | id                             | 
| email          | email.address                  |
| phoneNumber    | phoneNumber.text               | 
| familyName     | name.familyName                |
| givenName      | name.givenName                 |
| fullName       | name.fullName                  |
| organization   | organization.orgName           |
| department     | organization.orgDepartment     |
| title          | organization.orgTitle          |
| jobDescription | organization.orgJobDescription |
| where          | where.valueString              |
| notes          | content                        |

`phoneNumber` can be extended by adding a Google `rel` or a label to the 
property name in the config.json AttributeMap. For example:
`phoneNumber,http://schemas.google.com/g/2005#work` or
`phoneNumber,Personal Phone`. If neither are supplied, the "work" rel will be
applied and the `primary` attribute will be set.

Consult the [Google API reference](https://developers.google.com/gdata/docs/2.0/elements#gdContactKind) for details.

Below is an example of the destination configuration required for Google Shared
Contacts:

```json
{
  "Destination": {
    "Type": "GoogleContacts",
    "DisableAdd": false,
    "DisableUpdate": false,
    "DisableDelete": false,
    "ExtraJSON": {
      "BatchSize": 10,
      "BatchDelaySeconds": 3,
      "DelegatedAdminEmail": "delegated-admin@example.com",
      "Domain": "example.com",
      "GoogleAuth": {
        "type": "service_account",
        "project_id": "abc-theme-123456",
        "private_key_id": "abc123",
        "private_key": "-----BEGIN PRIVATE KEY-----\nMIIabc...\nabc...\n...xyz\n-----END PRIVATE KEY-----\n",
        "client_email": "my-sync-bot@abc-theme-123456.iam.gserviceaccount.com",
        "client_id": "123456789012345678901",
        "auth_uri": "https://accounts.google.com/o/oauth2/auth",
        "token_uri": "https://oauth2.googleapis.com/token",
        "auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",
        "client_x509_cert_url": "https://www.googleapis.com/robot/v1/metadata/x509/my-sync-bot%40abc-theme-123456.iam.gserviceaccount.com"
      }            
    }
  },
  "AttributeMap": [
    {
      "Source": "email",
      "Destination": "email",
      "required": true
    },
    {
      "Source": "phoneNumber",
      "Destination": "phoneNumber",
      "required": false
    },
    {
      "Source": "last_name",
      "Destination": "familyName",
      "required": true
    },
    {
      "Source": "first_name",
      "Destination": "givenName",
      "required": true
    },
    {
      "Source": "display_name",
      "Destination": "fullName",
      "required": false
    },
    {
      "Source": "organization",
      "Destination": "organization",
      "required": false
    },
    {
      "Source": "department",
      "Destination": "department",
      "required": false
    },
    {
      "Source": "title",
      "Destination": "title",
      "required": false
    },
    {
      "Source": "job_description",
      "Destination": "jobDescription",
      "required": false
    },
    {
      "Source": "where",
      "Destination": "where",
      "required": false
    }
  ]
}
```

Note: `Source` fields should be adjusted to fit the actual source adapter.

Configurations for `BatchSize`, `BatchDelaySeconds`, `DisableAdd`, `DisableUpdate`, and `DisableDelete` are all optional with defaults as shown in example.

### Google Groups
This destination is useful for keeping Google Groups in sync with reports from a personnel system. Below is an example 
of the destination configuration required for Google Groups:

```json
{
  "Destination": {
    "Type": "GoogleGroups",
    "ExtraJSON": {
      "BatchSize": 10,
      "BatchDelaySeconds": 3,
      "DelegatedAdminEmail": "delegated-admin@domain.com",
      "GoogleAuth": {
        "type": "service_account",
        "project_id": "abc-theme-123456",
        "private_key_id": "abc123",
        "private_key": "-----BEGIN PRIVATE KEY-----\nMIIabc...\nabc...\n...xyz\n-----END PRIVATE KEY-----\n",
        "client_email": "my-sync-bot@abc-theme-123456.iam.gserviceaccount.com",
        "client_id": "123456789012345678901",
        "auth_uri": "https://accounts.google.com/o/oauth2/auth",
        "token_uri": "https://oauth2.googleapis.com/token",
        "auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",
        "client_x509_cert_url": "https://www.googleapis.com/robot/v1/metadata/x509/my-sync-bot%40abc-theme-123456.iam.gserviceaccount.com"
      }
    }
  },
  "AttributeMap": [
    {
      "Source": "First_Name",
      "Destination": "givenName",
      "required": true
    },
    {
      "Source": "Last_Name",
      "Destination": "sn",
      "required": true
    },
    {
      "Source": "Email",
      "Destination": "mail",
      "required": true
    }
  ],
  "SyncSets": [
    {
      "Name": "Sync from personnel to Google Groups",
      "Source": {
          "Path": ["/user-report"]
      },
      "Destination": {
          "GroupEmail": "group1@groups.domain.com",
          "Owners": ["person_a@domain.com","person_b@domain.com"],
          "Managers": ["another_person@domain.com", "yet-another-person@domain.com"],
          "ExtraOwners": ["google-admin@domain.com"],
          "DisableAdd": false,
          "DisableUpdate": false,
          "DisableDelete": false
      }
    }
  ]
}
```

Note: `Source` fields should be adjusted to fit the actual source adapter.

Configurations for `BatchSize`, `BatchDelaySeconds`, `DisableAdd`, `DisableUpdate`, and `DisableDelete` are all optional with defaults as shown in example.

### Google Sheets
The Google Sheets destination creates a copy of the source data in a Google Sheets
document.

If any of the disable options, DisableAdd, DisableDelete, or DisableUpdate are
set to true, no sync will be performed.

There must be at least two rows in the sheet to begin with. The first row must
be pre-filled with field names. The second row must be present, but will be
ignored and may be overwritten.

The entire sheet will be overwritten with new data on every sync

If not specified in the configuration, the sheet updated is "Sheet1"

Example config:
```json
{
  "Destination": {
    "Type": "GoogleSheets",
    "ExtraJSON": {
      "DelegatedAdminEmail": "admin@example.com",
      "GoogleAuth": {
        "type": "service_account",
        "project_id": "abc-theme-123456",
        "private_key_id": "abc123",
        "private_key": "-----BEGIN PRIVATE KEY-----\nMIIabc...\nabc...\n...xyz\n-----END PRIVATE KEY-----\n",
        "client_email": "my-sync-bot@abc-theme-123456.iam.gserviceaccount.com",
        "client_id": "123456789012345678901",
        "auth_uri": "https://accounts.google.com/o/oauth2/auth",
        "token_uri": "https://oauth2.googleapis.com/token",
        "auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",
        "client_x509_cert_url": "https://www.googleapis.com/robot/v1/metadata/x509/my-sync-bot%40abc-theme-123456.iam.gserviceaccount.com"
      }            
    }
  },
  "AttributeMap": [
    {
      "Source": "email",
      "Destination": "email"
    },
    {
      "Source": "employee_id",
      "Destination": "employee_id"
    }
  ],
  "SyncSets": [
    {
      "Name": "Sync from Xyz API to Google Sheets",
      "Source": {
        "Paths": ["/user"]
      },
      "Destination": {
        "SheetID": "putAnActualSheetIDHerejD70xAjqPnOCHlDK3YomH",
        "SheetName": "Sheet2"
      }
    }
  ]
}
```

Note: `Source` fields should be adjusted to fit the actual source adapter.

### Google Users
This destination can update User records in the Google Directory. Create and
delete are not yet implemented. The compare attribute is `email` (`primaryEmail`).
A limited subset of user properties are available to be updated. 

| property   | Google property | Google sub-property | Google type  |
|------------|-----------------|---------------------|--------------|
| id         | externalIds     | value               | organization |
| email      | primaryEmail    |                     |              |
| area       | locations       | area                | desk         |
| costCenter | organizations*  | costCenter          | (not set)    |
| department | organizations*  | department          | (not set)    |
| title      | organizations*  | title               | (not set)    |
| phone      | phones          | value               |              |
| manager    | relations       | value               | manager      |
| familyName | name            | familyName          | n/a          |
| givenName  | name            | givenName           | n/a          |

Custom schema properties can be added using dot notation. For example, a
custom property with Field name `Building` in the custom schema `Location`
is represented as `Location.Building`. __NOTE:__ Spaces in schema name and field 
name should be replaced by underscores. Google may also append a number on field
names, e.g. "Building_2", in which case the configuration should be `Location.Building_2`
             
Phone types are represented by separating the property name from its type with
a comma (`,`). For example: `phone,home` or `phone,work`. Multiple phones of the
same type can be referenced by adding a tilde (`~`) and a number. For example:
`phone,work` and `phone,work~1`. Types other than those defined by the
[Google API spec](https://developers.google.com/admin-sdk/directory/reference/rest/v1/users#User.FIELDS.phones)
should be referenced using a custom type as follows: `phone,custom,sat`.

__\* CAUTION:__ updating any field in `organizations` will overwrite all
existing organizations
             
Following is an example configuration listing all available fields:

```json
{
  "Destination": {
    "Type": "GoogleUsers",
    "ExtraJSON": {
      "BatchSize": 10,
      "BatchDelaySeconds": 3,
      "DelegatedAdminEmail": "admin@example.com",
      "GoogleAuth": {
        "type": "service_account",
        "project_id": "abc-theme-123456",
        "private_key_id": "abc123",
        "private_key": "-----BEGIN PRIVATE KEY-----\nMIIabc...\nabc...\n...xyz\n-----END PRIVATE KEY-----\n",
        "client_email": "my-sync-bot@abc-theme-123456.iam.gserviceaccount.com",
        "client_id": "123456789012345678901",
        "auth_uri": "https://accounts.google.com/o/oauth2/auth",
        "token_uri": "https://oauth2.googleapis.com/token",
        "auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",
        "client_x509_cert_url": "https://www.googleapis.com/robot/v1/metadata/x509/my-sync-bot%40abc-theme-123456.iam.gserviceaccount.com"
      }            
    }
  },
  "AttributeMap": [
    {
      "Source": "email",
      "Destination": "email",
      "required": true
    },
    {
      "Source": "last_name",
      "Destination": "familyName",
      "required": true
    },
    {
      "Source": "first_name",
      "Destination": "givenName",
      "required": true
    },
    {
      "Source": "id",
      "Destination": "id",
      "required": false
    },
    {
      "Source": "phone",
      "Destination": "phone",
      "required": false
    },
    {
      "Source": "area",
      "Destination": "area",
      "required": false
    },
    {
      "Source": "building",
      "Destination": "Location.Building",
      "required": false
    },
    {
      "Source": "cost_center",
      "Destination": "costCenter",
      "required": false
    },
    {
      "Source": "department",
      "Destination": "department",
      "required": false
    },
    {
      "Source": "title",
      "Destination": "title",
      "required": false
    },
    {
      "Source": "manager",
      "Destination": "manager",
      "required": false
    }
  ]
}
```

Note: `Source` fields should be adjusted to fit the actual source adapter.

#### Google Service Account Configuration

(see https://stackoverflow.com/questions/53808710/authenticate-to-google-admin-directory-api#answer-53808774 and
 https://developers.google.com/admin-sdk/reports/v1/guides/delegation)

In the [Google Developer Console](https://console.developers.google.com) ...
* Enable the appropriate API for the Service Account in the Google APIs
 Developer Console, APIs and Services, Enable APIS And Services.
  * For the Google Users adapter, enable "Admin SDK"
  * For the Google Groups adapter, enable "Admin SDK"
  * For the Google Contacts adapter, enable "Contacts API"
  * For the Google Sheets adapter, enable "Google Sheets API"
* Create a new Service Account and a corresponding JSON credential file, which should contain something like this:

```json
  {
    "type": "service_account",
    "project_id": "abc-theme-123456",
    "private_key_id": "abc123",
    "private_key": "-----BEGIN PRIVATE KEY-----\nMIIabc...\nabc...\n...xyz\n-----END PRIVATE KEY-----\n",
    "client_email": "my-sync-bot@abc-theme-123456.iam.gserviceaccount.com",
    "client_id": "123456789012345678901",
    "auth_uri": "https://accounts.google.com/o/oauth2/auth",
    "token_uri": "https://oauth2.googleapis.com/token",
    "auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",
    "client_x509_cert_url": "https://www.googleapis.com/robot/v1/metadata/x509/my-sync-bot%40abc-theme-123456.iam.gserviceaccount.com"
  }
```

These contents will need to be copied into the `config.json` file as the value of the `GoogleAuth` key under 
`Destination`/`ExtraJSON`.

In Google Admin, Security, [API Controls](https://admin.google.com/ac/owl?hl=en) ...
* Manage Domain-wide Delegation
* Add the appropriate API Scopes to the Service Account. Use the numeric `client_id`.
* API Scopes required for Google Groups are: `https://www.googleapis.com/auth/admin.directory.group` and
  `https://www.googleapis.com/auth/admin.directory.group.member`
* The API Scope required for Google Contacts is: `https://www.google.com/m8/feeds/contacts/`
* The API Scope required for Google User Directory is: `https://www.googleapis.com/auth/admin.directory.user`
* Google Sheets does not require Domain-wide Delegation. Instead, share the sheet with the service account. Note: it will say the user is not in the organization. This warning can be ignored. If you do add a `DelegatedAdminEmail` address, you must use the API Scope https://www.googleapis.com/auth/spreadsheets which will grant admin access to all sheets.

The sync job will need to use the Service Account credentials to impersonate another user that has
appropriate domain privileges and who has logged in at least once into Google Workspace and
accepted the terms and conditions. The email address for this user should be stored in the `config.json`
as the `DelegatedAdminEmail` value under `Destination`/`ExtraJSON`.

### SolarWinds WebHelpDesk


```json
{
  "AttributeMap": [
      {
        "Source": "FIRST_NAME",
        "Destination": "firstName",
        "required": true,
        "CaseSensitive": true
      },
      {
        "Source": "LAST_NAME",
        "Destination": "lastName",
        "required": true,
        "CaseSensitive": true
      },
      {
        "Source": "EMAIL",
        "Destination": "email",
        "required": true,
        "CaseSensitive": false
      },
      {
        "Source": "USER_NAME",
        "Destination": "username",
        "required": true,
        "CaseSensitive": false
      },
      {
        "Source": "Staff_ID",
        "Destination": "employmentStatus"
      }
  ],
  "Destination": {
    "Type": "WebHelpDesk",
    "ExtraJSON": {
      "URL": "https://whd.mycompany.com/helpdesk/WebObjects/Helpdesk.woa",
      "Username": "syncuser",
      "Password": "apitoken",
      "ListClientsPageLimit": 100,
      "BatchSize": 50,
      "BatchDelaySeconds": 60
    }
  }
}
```

`ListClientsPageLimit`, `BatchSize` and `BatchDelaySeconds` are optional. Their defaults are as shown in the example config.

## AttributeMap

The `AttributeMap` section of the config file lists the data attributes to be synchronized from Source to Destination. It has the following parameters:

- `Source`

This is the name of an attribute as found in the [Source](#Sources) data provider. It is a required parameter.

- `Destination`

This is the name of an attribute as found in the [Destination](#Destinations) data provider. It is a required parameter.

- `Required`

If this parameter is included and contains `true`, then any source record that does not include this attribute is dropped from the data set.

- `CaseSensitive`

If this parameter is included and contains `true`, then the comparison between source and destination data values does not match if case is different.

- `Expression`

Optional regular expression string to use with the `Replace` parameter. Use groups delimited by parentheses to manipulate data as it is transferred from Source to Destination.

Example: `(.*)`

- `Replace`

Optional replacement string to manipulate Source data before writing to the Destination. Group identifiers like `$1` can be used to reference groups defined in the `Expression` parameter.

Example: `https://example.com?query=$1`

## SyncSets

At least one configuration set must be defined in SyncSets. This is where the API path or other adapter-specific options are specified.

- `Name`

The name of the SyncSet, used for log messages

- `Source`

Data source configuration. Refer to the adapter documentation above for details.

- `Destination`

Data destination configuration. Refer to the adapter documentation above for details.

- `Disable`

If set to `true`, this SyncSet will be skipped.

# Other notes

### Exporting logs from CloudWatch

The log messages in CloudWatch can be viewed on the AWS Management Console. If
an exported text or json file is needed, the AWS CLI tool can be used as
follows:

```shell script
aws configure
aws logs get-log-events \
   --log-group-name "/aws/lambda/lambda-name" \
   --log-stream-name '2019/11/14/[$LATEST]0123456789abcdef0123456789abcdef' \
   --output text \
   --query 'events[*].message'
```

Replace `/aws/lambda/lambda-name` with the actual log group name and 
`2019/11/14/[$LATEST]0123456789abcdef0123456789abcdef` with the actual log
stream. Note the single quotes around the log stream name to prevent the shell
from interpreting the `$` character. `--output text` can be changed to 
`--output json` if desired. Timestamps are available if needed, but omitted
in this example by the `--query` string.
