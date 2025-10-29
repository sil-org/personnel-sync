# Running personnel-sync as a Lambda process

This personnel-sync application is a good use case for Serverless operation. As
a primarily scheduled job, running it in a Serverless/Lambda environment is a
great way to reduce infrastructure overhead.

### Important warning

This application is configured using a `config.json` file that may include
secrets. Be sure not to store this file in a public repository. 

## Setup

The contents of this `lambda-example` directory are intended to be copied to
your own filesystem and repository. In this directory are a couple files for use
with Github Actions. If you don't use Github Actions you can remove them and
replace them with whatever is appropriate for your CI/CD provider.

After downloading the files, copy `config.example.json` to `config.json` and
edit as needed. When running locally, copy `.env.example` to `.env` and insert AWS
credentials for AWS CDK to use to deploy the Lambda function. 

Due to dependencies on Go to build the application and AWS CDK to interact
with AWS to create and deploy the Lambda the easiest way to do all this is using
the included `Dockerfile` and `docker compose`.

Run `make deploy`. This will build the Docker image, run `go get` inside the
container, build the Go binary, and use CDK to deploy the Lambda function
to `dev`, but you can update the `.env` file to change the stage as desired. 

By default, it is configured to only deploy to _production_ when changes are pushed
to the main branch.
