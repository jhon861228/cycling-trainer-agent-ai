package repository

import (
	"context"
	"cycling-trainer-agent-ai/backend/models"
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// Repository defines the interface for all DynamoDB operations
type Repository interface {
	GetUser(ctx context.Context, email string) (*models.User, error)
	CreateUser(ctx context.Context, user *models.User) error
	UpdateUser(ctx context.Context, user *models.User) error

	SaveWorkoutDay(ctx context.Context, workout *models.WorkoutDay) error
	GetWorkouts(ctx context.Context, email string, activeOnly bool) ([]models.WorkoutDay, error)
	GetWorkout(ctx context.Context, email string, planID string, day string) (*models.WorkoutDay, error)
	DeactivateActiveWorkouts(ctx context.Context, email string) error
	SaveFTPRecord(ctx context.Context, record *models.FTPRecord) error
	ToggleWorkoutDone(ctx context.Context, email, planID, day string, completed bool) error
}

type dynamoRepository struct {
	client    *dynamodb.Client
	tableName string
}

// NewDynamoRepository creates a new instance of the repository
func NewDynamoRepository(client *dynamodb.Client, tableName string) Repository {
	return &dynamoRepository{
		client:    client,
		tableName: tableName,
	}
}

func (r *dynamoRepository) GetUser(ctx context.Context, email string) (*models.User, error) {
	result, err := r.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: "USER#" + email},
			"SK": &types.AttributeValueMemberS{Value: "PROFILE"},
		},
	})
	if err != nil {
		log.Printf("[DB ERROR] GetUser failed for %s: %v", email, err)
		return nil, fmt.Errorf("getting user: %w", err)
	}
	if result.Item == nil {
		return nil, nil // Not found
	}

	var user models.User
	if err := attributevalue.UnmarshalMap(result.Item, &user); err != nil {
		return nil, fmt.Errorf("unmarshaling user: %w", err)
	}
	return &user, nil
}

func (r *dynamoRepository) CreateUser(ctx context.Context, user *models.User) error {
	item, err := attributevalue.MarshalMap(user)
	if err != nil {
		return fmt.Errorf("marshaling user: %w", err)
	}

	_, err = r.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName:           aws.String(r.tableName),
		Item:                item,
		ConditionExpression: aws.String("attribute_not_exists(PK)"),
	})
	if err != nil {
		log.Printf("[DB ERROR] CreateUser failed for %s: %v", user.Email, err)
		return fmt.Errorf("putting user: %w", err)
	}
	return nil
}

func (r *dynamoRepository) UpdateUser(ctx context.Context, user *models.User) error {
	item, err := attributevalue.MarshalMap(user)
	if err != nil {
		return fmt.Errorf("marshaling user: %w", err)
	}

	_, err = r.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(r.tableName),
		Item:      item,
	})
	if err != nil {
		log.Printf("[DB ERROR] UpdateUser failed for %s: %v", user.Email, err)
		return fmt.Errorf("updating user: %w", err)
	}
	return nil
}

func (r *dynamoRepository) SaveWorkoutDay(ctx context.Context, workout *models.WorkoutDay) error {
	item, err := attributevalue.MarshalMap(workout)
	if err != nil {
		return fmt.Errorf("marshaling workout: %w", err)
	}

	_, err = r.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(r.tableName),
		Item:      item,
	})
	if err != nil {
		log.Printf("[DB ERROR] SaveWorkoutDay failed for %s (SK: %s): %v", workout.PK, workout.SK, err)
		return fmt.Errorf("putting workout day: %w", err)
	}
	return nil
}

func (r *dynamoRepository) GetWorkouts(ctx context.Context, email string, activeOnly bool) ([]models.WorkoutDay, error) {
	pk := "USER#" + email
	input := &dynamodb.QueryInput{
		TableName:              aws.String(r.tableName),
		KeyConditionExpression: aws.String("PK = :pk AND begins_with(SK, :sk)"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk": &types.AttributeValueMemberS{Value: pk},
			":sk": &types.AttributeValueMemberS{Value: "DAY#"},
		},
	}

	if activeOnly {
		input.FilterExpression = aws.String("is_active = :active")
		input.ExpressionAttributeValues[":active"] = &types.AttributeValueMemberBOOL{Value: true}
	} else {
		input.FilterExpression = aws.String("is_active = :active")
		input.ExpressionAttributeValues[":active"] = &types.AttributeValueMemberBOOL{Value: false}
	}

	result, err := r.client.Query(ctx, input)
	if err != nil {
		log.Printf("[DB ERROR] Query workouts failed for %s: %v", email, err)
		return nil, fmt.Errorf("querying workouts: %w", err)
	}

	var workouts []models.WorkoutDay
	if err := attributevalue.UnmarshalListOfMaps(result.Items, &workouts); err != nil {
		return nil, fmt.Errorf("unmarshaling workouts: %w", err)
	}
	return workouts, nil
}

func (r *dynamoRepository) GetWorkout(ctx context.Context, email string, planID string, day string) (*models.WorkoutDay, error) {
	result, err := r.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: "USER#" + email},
			"SK": &types.AttributeValueMemberS{Value: fmt.Sprintf("DAY#%s#%s", day, planID)},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("getting workout item: %w", err)
	}
	if result.Item == nil {
		return nil, nil // Not found
	}

	var workout models.WorkoutDay
	if err := attributevalue.UnmarshalMap(result.Item, &workout); err != nil {
		return nil, fmt.Errorf("unmarshaling workout: %w", err)
	}
	return &workout, nil
}

func (r *dynamoRepository) DeactivateActiveWorkouts(ctx context.Context, email string) error {
	pk := "USER#" + email

	activeRecords, err := r.client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(r.tableName),
		KeyConditionExpression: aws.String("PK = :pk AND begins_with(SK, :sk)"),
		FilterExpression:       aws.String("is_active = :active"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk":     &types.AttributeValueMemberS{Value: pk},
			":sk":     &types.AttributeValueMemberS{Value: "DAY#"},
			":active": &types.AttributeValueMemberBOOL{Value: true},
		},
	})
	if err != nil {
		log.Printf("[DB ERROR] Query active records for deactivation failed for %s: %v", email, err)
		return fmt.Errorf("querying active records for deactivation: %w", err)
	}

	if activeRecords != nil {
		for _, item := range activeRecords.Items {
			_, err = r.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
				TableName: aws.String(r.tableName),
				Key: map[string]types.AttributeValue{
					"PK": item["PK"],
					"SK": item["SK"],
				},
				UpdateExpression: aws.String("SET is_active = :inactive"),
				ExpressionAttributeValues: map[string]types.AttributeValue{
					":inactive": &types.AttributeValueMemberBOOL{Value: false},
				},
			})
			if err != nil {
				// We log and continue, or we could return error
				fmt.Printf("!! Warning: failed to deactivate item: %v\n", err)
			}
		}
	}
	return nil
}

func (r *dynamoRepository) SaveFTPRecord(ctx context.Context, record *models.FTPRecord) error {
	item, err := attributevalue.MarshalMap(record)
	if err != nil {
		return fmt.Errorf("marshaling ftp record: %w", err)
	}

	_, err = r.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(r.tableName),
		Item:      item,
	})
	if err != nil {
		log.Printf("[DB ERROR] SaveFTPRecord failed for %s: %v", record.PK, err)
		return fmt.Errorf("putting ftp record: %w", err)
	}
	return nil
}

func (r *dynamoRepository) ToggleWorkoutDone(ctx context.Context, email, planID, day string, completed bool) error {
	pk := "USER#" + email
	sk := fmt.Sprintf("DAY#%s#%s", day, planID)

	_, err := r.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: pk},
			"SK": &types.AttributeValueMemberS{Value: sk},
		},
		UpdateExpression: aws.String("SET completed = :val"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":val": &types.AttributeValueMemberBOOL{Value: completed},
		},
	})
	if err != nil {
		log.Printf("[DB ERROR] ToggleWorkoutDone failed for %s / %s: %v", pk, sk, err)
		return fmt.Errorf("toggling workout done: %w", err)
	}

	log.Printf("[DB] Workout marked as %v for %s (SK: %s)", completed, email, sk)
	return nil
}
