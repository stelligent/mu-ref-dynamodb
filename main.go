package main

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/gin-gonic/gin"
	"os"
	"strconv"
	"time"
)

type Timer struct {
	TimerId          string
	StartTime        int64
	Now              int64
	Seconds          int
	SecondsRemaining int
}

type TimerDatabase struct {
	Db        *dynamodb.DynamoDB
	Region    string
	TableName string
}

func makeTimerDatabase() *TimerDatabase {
	region := os.Getenv("REGION")
	config := &aws.Config{Region: aws.String(region)}
	return &TimerDatabase{
		Db:        dynamodb.New(session.New(), config),
		Region:    os.Getenv("REGION"),
		TableName: os.Getenv("TABLE_NAME"),
	}
}

func (timers *TimerDatabase) Find(timerId string) (*Timer, error) {
	response, err := timers.Db.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String(timers.TableName),
		Key: map[string]*dynamodb.AttributeValue{
			"Timer": {
				S: aws.String(timerId),
			},
		},
	})
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	if response == nil || response.Item == nil {
		return nil, nil
	}
	startTime, err := strconv.ParseInt(*response.Item["StartTime"].N, 10, 64)
	if err != nil {
		return nil, err
	}
	seconds, err := strconv.Atoi(*response.Item["Seconds"].N)
	if err != nil {
		return nil, err
	}
	now := time.Now().Unix()
	secondsRemaining := int(startTime + int64(seconds) - now)
	return &Timer{
		TimerId:          *response.Item["Timer"].S,
		StartTime:        startTime,
		Seconds:          seconds,
		Now:              now,
		SecondsRemaining: secondsRemaining,
	}, nil
}

func (timers *TimerDatabase) Set(timerId string, seconds int) (*Timer, error) {
	startTime := time.Now().Unix()
	startTimeString := strconv.FormatInt(startTime, 10)
	secondsString := strconv.Itoa(seconds)
	_, err := timers.Db.PutItem(&dynamodb.PutItemInput{
		TableName: aws.String(timers.TableName),
		Item: map[string]*dynamodb.AttributeValue{
			"Timer": {
				S: aws.String(timerId),
			},
			"StartTime": {
				N: aws.String(startTimeString),
			},
			"Seconds": {
				N: aws.String(secondsString),
			},
		},
	})
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	return &Timer{
		TimerId:          timerId,
		StartTime:        startTime,
		Now:              startTime,
		Seconds:          seconds,
		SecondsRemaining: seconds,
	}, nil
}

func healthCheck(c *gin.Context) {
	c.JSON(200, gin.H{"Status": "OK"})
}

func checkTimer(timers *TimerDatabase) func(c *gin.Context) {
	return func(c *gin.Context) {
		timer, err := timers.Find(c.Param("timer"))
		if err != nil {
			c.JSON(500, gin.H{
				"Message": "Internal error",
			})
			return
		}
		if timer == nil {
			c.JSON(404, gin.H{
				"Message": "Timer not found",
			})
			return
		}
		if timer.SecondsRemaining < 0 {
			c.JSON(504, gin.H{
				"Message": "Timer expired",
			})
			return
		}
		c.JSON(200, timer)
	}
}

func setTimer(timers *TimerDatabase) func(c *gin.Context) {
	return func(c *gin.Context) {
		seconds, err := strconv.Atoi(c.Param("seconds"))
		if err != nil {
			c.JSON(400, gin.H{
				"Message": "Invalid number of seconds",
			})
			return
		}
		timer, err := timers.Set(c.Param("timer"), seconds)
		if err != nil {
			c.JSON(500, gin.H{
				"Message": "Internal error",
			})
			return
		}
		c.JSON(200, timer)
	}
}

func main() {
	timers := makeTimerDatabase()
	r := gin.Default()
	r.GET("/", healthCheck)
	r.GET("/:timer", checkTimer(timers))
	r.GET("/:timer/:seconds", setTimer(timers))
	r.Run()
}
