provider "aws" {
  region = "us-east-1"
}

resource "aws_dynamodb_table" "cycling_app_table" {
  name           = "CyclingTrainerData"
  billing_mode   = "PAY_PER_REQUEST"
  hash_key       = "PK" # Primary Key (e.g., USER#email)
  range_key      = "SK" # Sort Key (e.g., PROFILE, WORKOUT#id, FTP#timestamp)

  attribute {
    name = "PK"
    type = "S"
  }

  attribute {
    name = "SK"
    type = "S"
  }

  tags = {
    Project = "CyclingTrainerAgent"
  }
}

resource "aws_iam_role" "lambda_exec_role" {
  name = "cycling_trainer_lambda_role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Action = "sts:AssumeRole"
      Effect = "Allow"
      Sid    = ""
      Principal = {
        Service = "lambda.amazonaws.com"
      }
      }
    ]
  })
}

resource "aws_iam_role_policy_attachment" "lambda_policy" {
  role       = aws_iam_role.lambda_exec_role.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"
}

resource "aws_iam_policy" "dynamodb_access" {
  name        = "CyclingTrainerDynamoDBAccess"
  description = "Access to DynamoDB for Cycling Trainer App"

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = [
          "dynamodb:GetItem",
          "dynamodb:PutItem",
          "dynamodb:UpdateItem",
          "dynamodb:Query",
          "dynamodb:DeleteItem"
        ]
        Effect   = "Allow"
        Resource = aws_dynamodb_table.cycling_app_table.arn
      }
    ]
  })
}

resource "aws_iam_role_policy_attachment" "lambda_dynamodb" {
  role       = aws_iam_role.lambda_exec_role.name
  policy_arn = aws_iam_policy.dynamodb_access.arn
}
