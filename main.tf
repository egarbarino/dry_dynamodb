provider "aws" {
  profile = "dynamodb_profile"
  region  = "eu-west-2"
}

resource "aws_dynamodb_table" "dynamodb-table-users" {
  name           = "users"
  billing_mode   = "PROVISIONED"
  read_capacity  = 2 
  write_capacity = 2
  hash_key       = "id"
//  range_key      = "GameTitle"

  attribute {
    name = "id"
    type = "S"
  }

  attribute {
    name = "email"
    type = "S"
  }
/*
  ttl {
    attribute_name = "TimeToExist"
    enabled        = false
  }
*/
  global_secondary_index {
    name               = "users_by_email"
    hash_key           = "email"
    write_capacity     = 2 
    read_capacity      = 2 
    projection_type    = "INCLUDE"
    non_key_attributes = ["email"]
  }
  /*
  tags = {
    Name        = "dynamodb-table-1"
    Environment = "production"
  }
  */
}

resource "aws_dynamodb_table" "dynamodb-table-guests" {
  name           = "guests"
  billing_mode   = "PROVISIONED"
  read_capacity  = 2 
  write_capacity = 2
  hash_key       = "list_id"
  range_key       = "user_id"

  attribute {
    name = "list_id"
    type = "S"
  }

  attribute {
    name = "user_id"
    type = "S"
  }

  global_secondary_index {
    name               = "guests_by_user_id"
    hash_key           = "user_id"
    range_key          = "list_id"
    write_capacity     = 2 
    read_capacity      = 2 
    projection_type    = "KEYS_ONLY"
  }
}
resource "aws_dynamodb_table" "dynamodb-table-lists" {
  name           = "lists"
  billing_mode   = "PROVISIONED"
  read_capacity  = 2 
  write_capacity = 2
  hash_key       = "id"

  attribute {
    name = "id"
    type = "S"
  }

  attribute {
    name = "user_id"
    type = "S"
  }

  global_secondary_index {
    name               = "list_by_user_id"
    hash_key           = "user_id"
    write_capacity     = 2 
    read_capacity      = 2 
    projection_type    = "INCLUDE"
    non_key_attributes = ["id","title"]
  }
}

resource "aws_dynamodb_table" "dynamodb-table-items" {
  name           = "items"
  billing_mode   = "PROVISIONED"
  read_capacity  = 2 
  write_capacity = 2
  hash_key       = "list_id"
  range_key      = "datetime"

  attribute {
    name = "list_id"
    type = "S"
  }

  attribute {
    name = "datetime"
    type = "S"
  }

}
