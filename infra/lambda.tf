resource "aws_lambda_function" "cycling_api" {
  filename      = "lambda.zip"
  function_name = "cycling-trainer-api"
  role          = aws_iam_role.lambda_exec_role.arn
  handler       = "main"
  runtime       = "provided.al2023"

  environment {
    variables = {
      DYNAMODB_TABLE = aws_dynamodb_table.cycling_app_table.name
      GEMINI_API_KEY = var.gemini_api_key
    }
  }

  # This is a placeholder for the actual zip file
  lifecycle {
    ignore_changes = [filename]
  }
}

resource "aws_lambda_function_url" "api_url" {
  function_name      = aws_lambda_function.cycling_api.function_name
  authorization_type = "NONE"

  cors {
    allow_credentials = true
    allow_origins     = ["*"]
    allow_methods     = ["*"]
    allow_headers     = ["date", "keep-alive", "content-type"]
    max_age           = 86400
  }
}

output "lambda_url" {
  value = aws_lambda_function_url.api_url.function_url
}
