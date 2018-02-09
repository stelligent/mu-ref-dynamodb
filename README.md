# mu-ref-dynamodb

## Overview 

A simple web service that uses a DynamoDB table. The service is written in go, packaged in a Docker image, and deployed onto ECS with mu: http://getmu.io/

This is a long running service that was inspired by this one implemented using Lambda: https://alestic.com/2015/07/timercheck-scheduled-events-monitoring/

## Prerequisites

* Configure your AWS credentials. A simple way to do this to install the AWS command line tool, and then run `aws configure`. Instructions for installing the CLI can be found here: https://docs.aws.amazon.com/cli/latest/userguide/installing.html

* Install mu, using these instructions: https://github.com/stelligent/mu/wiki/Installation

## Service API

The web service has three endpoints:

* / This endpoint just returns OK, and is used as a health check endpoint
* /{timer-id}/{number-of-seconds} This endpoint creates or resets a timer for a number of seconds
* /{timer-id}  This endpoint checks the timer, returns 200 with a JSON payload if still running, a 504 if expired

The typical use case is to detect when some event has *not* occurred. For example, as part of a cron job
that should run every every hour, you can add a call to this service to set a timer for 70 minutes.
The timer will be reset every hour to run for 70 minutes.

Your monitoring software can poll the timer, and should receive a 200 response. If the cron job has failed to run, 
it will not reset the timer, and the check endpoint will eventually return a 504 error, and the monitoring system can raise an alert.

See here for an excellent description of the idea: https://alestic.com/2015/07/timercheck-scheduled-events-monitoring/

## Fork this repo

Fork this repo. Then edit the mu.yml file, and change the repo name to the be the name of your new repo. A CodePipeline will be created thatis started when changes are committed to this repo, so you want it to have your own repo name, not the stelligent one.

```
service:
  name: timercheck
  ...
  pipeline:
    source:
      repo: [Put your repo name here]
  ...
```

Commit this change and push to GitHub.

```
git add mu.yml
git commit -m'Use new repo'
git push origin master
```

## Deploy to acceptance environment

You will need a personal access token from GitHub that CodePipeline will use to access to your repo.
If you don't alredy have one of these, you can  go here to set one up: https://github.com/settings/tokens

```
mu pipeline up -t [GitHub Personal Token]
```

This will provision a continuous delivery pipeline that will deploy the service any time something is pushed to the GitHub repo.
The pipeline will also be started just after it is created.

The pipeline will provision an acceptance environment, and deploy the service there. See here for information about the environment mu will created: https://github.com/stelligent/mu/wiki/Environments

This example uses a mu extension that will also provision a DynamoDB table as part of the environment. See below for a description of the CloudFormation resources created by the mu extension.

You can monitor the progress of the pipeline by using this command:

```
mu svc show
```

Once the Acceptance stage Deploy action has a status of 'Succeeded', run the following command:
```
mu env show acceptance
```

This will report the BaseURL that you can use for curl or in a web browser.

## Try the service

Create a timer with this command:

```
curl [URL]/my-first-timer/20
```
You can replace `my-first-timer` with any string you want to use as a timer identifier. Each timer will be saved as a record
in the DynamoDB table.

Use the same name to check the timer:

```
curl [URL]/my-first-timer
```

This will return a 200 status code, along with a JSON payload of information about the state of the timer.
After 20 seconds the timer will expire, and a 504 status code will be returned the next time the check endpoint is invoked.


## Deploy to production environment

After successfully deploying to the acceptance environment, the pipeline will wait for a manual approval. If that is approved, the pipeline will deploy to the production environment. The first time this happens the production environment will be created. You can use the AWS console to approve or reject the deployment.

Go to the CodePipeline console, and follow the link for the `mu-timercheck` pipeline. Click the `Review` button and then you can either approve or reject the revision.


### Use the command line for the manual approval step

You can also use the CLI to approve or reject what was deployed to the acceptance environment. You will need a token from the pipeline state to do this. Use this command:
```
aws codepipeline get-pipeline-state --name mu-timercheck --query 'stageStates[?stageName==`Production`].actionStates[0][?actionName==`Approve`].latestExecution'
```

Copy the token from that output and use it for the next step.

### Command line approval

Use the token from the previous step in this command:

```
aws codepipeline put-approval-result --pipeline-name mu-timercheck --stage-name Production --action-name Approve --token [TOKEN] --result status=Approved,summary=Working
```


### Command line rejection

Use the token in this command to cancel the deployment pipeline, and not deploy to production:

```
aws codepipeline put-approval-result --pipeline-name mu-timercheck --stage-name Production --action-name Approve --token [TOKEN] --result status=Rejected,summary=Broke
```

## Monitor pipeline for production

You can monitor the progress of the pipeline by using the same command as before:

```
mu svc show
```

Once the Production stage Deploy action has a status of 'Succeeded', run the following command:
```
mu env show production
```
This reports the `Base URL` that you can use to interact with the production environment using curl or a web browser.

## Extension to create a DynamoDB table

The mu.yml file references the dynamodb subdirectory under extensions. The subdirectory contains a custom mu extension. The two files in that subdirectory match names of standard templates that mu uses to provision infrastructure. The CloudFormation resources and parameters in the extension files are merged with the CloudFormation provided by the standard templates. 
(It is also possible to completely replace the default mu CloudFormation, but that's not what we want here. See here for more info about extensions: https://github.com/stelligent/mu/wiki/Custom-CloudFormation#extensions.)

A DynamoDB table is created as part of each environment, so you end up with one in acceptance, and one in production.

The `service-iam.yml` template adds IAM actions to two different roles in each environment. One role is used by CloudFormation to actually create the table. The other is used by the ECS service, and gives it permission to read and write records in the table.

The `service-ecs.yml` template is used when creating the environment, and provisions the table.


## Cleanup

To cleanup you can run these commands:

```
mu environment terminate acceptance
mu environment terminate production
mu pipeline terminate
```

There are several CloudFormation stacks created by my that are shared by all environments and services: `mu-bucket-codedeploy` and `mu-bucket-codepipeline`. Because these are shared by all environments, you will need to delete these manually using the AWS console or CLI, once you have deleted all services and environments.

