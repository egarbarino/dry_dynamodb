# Introduction

This is a _dry_, project-based DynamoDB tutorial using Terraform, Python, and Go. It is dry in the sense that it is written in a bottom up fashion, facts are given a succinct account, and the low-level details are captured in the provided sample GitHub project, rather than as a blog narrative.

The related you-tube video is found here:

## Why DynamoDB?

DynamoDB is AWS' de facto, serverless database. While DynamoDB "only works on AWS", it has numerous advantages when compared with both SQL or non-SQL offerings (like MongoDB):

* **Serverless:** No servers to manage and no decisions about VM types.
* **Masterless:** No concepts of masters and slaves, or "read" versus "write" replicas.
* **Schemaless:** No need to pin down attributes in advance. 
* **Performance:** Measured in milliseconds and hundreds of requests per second.
* **Scale:** FAANG (Facebook, Amazon, Apple, Netflix, Google) scale level. 
* **Replication:** Planetary (multi-region) replication is supported.
* **Transactions:** No need to use a separate database for more "serious" use cases.
* **Pricing:** Both pay-per-use and "reserved"-type pricing models are available. 

## Why Terraform, Python, and Go?

The choice of Terraform, Python, and Go abides to the following principles:

* **Terraform:** Terraform is a ideal to define cloud resources in a declarative fashion. While DynamoDB tables may be defined from within any AWS SDK-supported language, using Terraform makes it easier to manage the table definitions through a clean model that is free of imperative programming language constructs.
* **Python:** We use Python to populate the tables with "fake", sample data. We presume that this step is of an _administrative_ nature, for which Go is inappropriate.  
* **Go:** This is the _application language_. The choice of Go offers these benefits:
  * _AWS Lambda_ and _containers_ suitability: Fast start-up time, low CPU/memory resource consumption, and the ability to use any Go release given that binaries are precompiled and there is no separate language runtime.
  * Maintainability: Large projects are easy to manage when using a statically-typed language like Go. In this sense, it is more convenient than NodeJS and Python---but similar to C# or Java. 
  
# Running the Project

These are the instructions for running the project from a local machine. Running it from an EC2 box is easier if the reader understands how role-based authorisation works.

## Set up DynamoDB

### IAM User

1. Create an AWS IAM user with Create/Read/Write access privileges to DynamoDB, for example, by attaching the policy named `AmazonDynamoDBFullAccess`.
2. Save the access key and the secret access key

### IAM User Profile

Set up a named profile for the new IAM user called `dynamodb_profile`. For example:

```
> aws configure --profile dynamodb_profile
AWS Access Key ID [None]: IAM_USER_ACCESS_KEY (Paste here)
AWS Secret Access Key [None]: IAM_USER_SECRET_ACCESS_KEY (Paste here)
Default region name [None]: eu-west-2
Default output format [None]: yaml
```

### Create DynamoDB Tables Using Terraform

Get Terraform from https://www.terraform.io/ if not already installed, switch to the repo's root, and apply `main.tf` as follows: 

```
> terraform apply
...
Plan: 4 to add, 0 to change, 0 to destroy.

Do you want to perform these actions?
  Terraform will perform the actions described above.
  Only 'yes' will be accepted to approve.

  Enter a value: yes
```

### Add Synthetic Data Using Python Script

Make sure Python3 is installed and add dependencies:

```
> pip3 install -r requirements.txt
```

Run the script:

```
> python3 init_database.py
```

# Run The Sample Todo List Application

First build the application so that dependencies are obtained:

```
> cd client_go
> go build 
```
## Using DynamoDB

```
> go run cmd/client/main.go 2> /tmp/log.txt
```

Logs can be checked on a separate console as follows:

```
> tail -f /tmp/log.txt
```

## Using the Memory Driver

Please note that only the `users` and `email` commands are implemented.

```
> go run cmd/client/main.go memory 2> /tmp/log.txt
```

# Exercises Left to the Reader

## Transactional List Delete

The `list delete` command is safe when it comes to taking care of items but not of guests. 

1. Try the slowing down the execution using the `slow SECONDS` command and add a guest in the midst of a list deletion process.
2. Fix the list delete code so that it deletes guests, in addition to items.
3. Fix the `guest create` code so that it fails if a list is undergoing deletion.



