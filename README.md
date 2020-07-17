---
title: A Dry Guide to DynamoDB
abstract: "Learning DynamoDB using Terraform, Python, and Go."
author: Ernesto Garbarino
date: 2020-06-12
---

# Introduction

This is a _dry_, project-based DynamoDB tutorial using Terraform, Python, and Go. It is dry in the sense that it is written in a bottom up fashion, facts are given a succinct account, and the low-level details are captured in the provided sample GitHub project, rather than as a blog narrative.

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
  


Notes

* Comments on region only

* Models
  * Eventually consistent
  * Strongly consistent

GetItem, Query, and Scan) provide a ConsistentRead parameter


* Encryption at REST
    * AWS owned Customer Master Key(CMK)
    * AWS managed CMK
    * Customer managed CMK

* Backups
    * On demand
    * Continuous Backups (Point-in-time Recovery)

* DynamoDB Time to Live (TTL)

* High availability and durability
    * SSD Storage
    * Automatically replicated across multiple AZs
    * Multi-region replication

* Concepts
  * Tables
  * Items
  * Attributes


* Data Types
  * Scalar Types – A scalar type can represent exactly one value. The scalar types are number, string, binary, Boolean, and null.

  * Document Types – A document type can represent a complex structure with nested attributes, such as you would find in a JSON document. The document types are list and map.

  * Set Types – A set type can represent multiple scalar values. The set types are string set, number set, and binary set.

* Partition Key
* Sort Key
* Composite Primary Key (Partition Key + Sort Key)

* Secondary Indexes
  * Local Secondary Index
  * Global Secondary Index

* Streams
  * Before
  * After
  * Lambda Triggers

* Pricing schemes
  * On-demand
  * Provisioned (default, free-tier eligible)

* API
  * Create
    * PutItem
    * BatchWriteItem

  * Read Data
    * GetItem
    * BatchGetItem
    * Query
    * Scan

  * Update Data
    * UpdateItem

  * Delete Data
    * DeleteItem
    * BatchWriteItem

  * Transactions
    * TransactWriteItems
    * TransactGetItems






