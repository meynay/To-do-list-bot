package main

import (
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

const (
	IDLE = iota
	ADD_TASK_PART1
	ADD_TASK_PART2
	START_POMO1
	START_POMO2
)

type Timer struct {
	Duration  time.Duration
	Starttime time.Time
	sync.Mutex
}

type POMOtask struct {
	gorm.Model
	TelID      int64  `gorm:"column:telid"`
	TaskName   string `gorm:"column:task"`
	NeededPomo int    `gorm:"column:pomos"`
	PomosDone  int    `gorm:"column:done"`
}

type App struct {
	Bot             *tgbotapi.BotAPI
	UserPlace       map[int64]int
	DB              *gorm.DB
	LastChatMessage map[int64]*tgbotapi.Message
	AddingTask      map[int64]POMOtask
	TaskToComplete  map[int64]uint
	Chatids         map[int64]int64
	PomoTimer       map[int64]*Timer
	StopChan        chan (bool)
}

func (app *App) ShowStartMessage(update tgbotapi.Update) {
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, fmt.Sprintf("Welcome %s %s to To Do List bot! We're glad to see you here. Press button below to start our journey", update.Message.From.FirstName, update.Message.From.LastName))
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Start using bot", "start"),
		),
	)
	msg.ReplyMarkup = keyboard
	sentMessage, _ := app.Bot.Send(msg)
	app.LastChatMessage[update.Message.From.ID] = &sentMessage
}

func (app *App) MainMenu(update tgbotapi.Update) {
	msg := tgbotapi.NewMessage(app.Chatids[update.CallbackQuery.From.ID], "Main Menu:\n Choose an option to continue using bot")
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Start POMO", "start pomo"),
			tgbotapi.NewInlineKeyboardButtonData("Tasks list", "tasks list"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Add task", "add task"),
			tgbotapi.NewInlineKeyboardButtonData("Completed tasks", "completed tasks"),
		),
	)
	msg.ReplyMarkup = keyboard
	sentMessage, _ := app.Bot.Send(msg)
	app.LastChatMessage[update.CallbackQuery.From.ID] = &sentMessage
}

func (app *App) AddTaskMesssage(update tgbotapi.Update) {
	msg := tgbotapi.NewMessage(app.Chatids[update.CallbackQuery.From.ID], "Enter new task name:")
	sentMessage, _ := app.Bot.Send(msg)
	app.LastChatMessage[update.CallbackQuery.From.ID] = &sentMessage
	app.UserPlace[update.CallbackQuery.From.ID] = ADD_TASK_PART1
}

func (app *App) AddTaskp1(update tgbotapi.Update) {
	task := POMOtask{
		TaskName: update.Message.Text,
		TelID:    update.Message.From.ID,
	}
	app.AddingTask[update.Message.From.ID] = task
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Enter count of pomos needed to complete this task:")
	sentMessage, _ := app.Bot.Send(msg)
	app.LastChatMessage[update.Message.From.ID] = &sentMessage
	app.UserPlace[update.Message.From.ID] = ADD_TASK_PART2
}

func (app *App) AddTaskp2(update tgbotapi.Update) {
	count, err := strconv.Atoi(update.Message.Text)
	if err != nil {
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Please enter the integer count (e.g. 1, 2, 3, ...):")
		sentMessage, _ := app.Bot.Send(msg)
		app.LastChatMessage[update.Message.From.ID] = &sentMessage
		return
	}
	task := app.AddingTask[update.Message.From.ID]
	task.NeededPomo = count
	task.PomosDone = 0
	app.DB.Create(&task)
	app.AddingTask[update.Message.From.ID] = POMOtask{}
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Task added to tasks list successfully!")
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Back to main menu", "start"),
		),
	)
	msg.ReplyMarkup = keyboard
	sentMessage, _ := app.Bot.Send(msg)
	app.LastChatMessage[update.Message.From.ID] = &sentMessage
	app.UserPlace[update.Message.From.ID] = IDLE
}

func (app *App) ShowTasksList(update tgbotapi.Update) {
	tasks := []POMOtask{}
	app.DB.Where("telid = ? AND done < pomos", update.CallbackQuery.From.ID).Find(&tasks)
	text := "Here is your ongoing tasks on bot:\n\n"
	for _, task := range tasks {
		percent := float64(task.PomosDone) / float64(task.NeededPomo) * 100
		text += fmt.Sprintf("\n%s\n%d pomos of %d done (%.2f%%)\n", task.TaskName, task.PomosDone, task.NeededPomo, percent)
	}
	msg := tgbotapi.NewMessage(app.Chatids[update.CallbackQuery.From.ID], text)
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Back to main menu", "start"),
		),
	)
	msg.ReplyMarkup = keyboard
	sentMessage, _ := app.Bot.Send(msg)
	app.LastChatMessage[update.CallbackQuery.From.ID] = &sentMessage
}

func (app *App) ShowCompleted(update tgbotapi.Update) {
	tasks := []POMOtask{}
	app.DB.Where("telid = ? AND done = pomos", update.CallbackQuery.From.ID).Find(&tasks)
	text := "Here is your completed tasks on bot:\n\n"
	for _, task := range tasks {
		text += fmt.Sprintf("\n%s ✅\n", task.TaskName)
	}
	msg := tgbotapi.NewMessage(app.Chatids[update.CallbackQuery.From.ID], text)
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Back to main menu", "start"),
		),
	)
	msg.ReplyMarkup = keyboard
	sentMessage, _ := app.Bot.Send(msg)
	app.LastChatMessage[update.CallbackQuery.From.ID] = &sentMessage
}

func (app *App) StartPOMO(update tgbotapi.Update) {
	tasks := []POMOtask{}
	app.DB.Where("telid = ? AND done < pomos", update.CallbackQuery.From.ID).Find(&tasks)
	msg := tgbotapi.NewMessage(app.Chatids[update.CallbackQuery.From.ID], "Choose a task to complete with this pomo")
	keyboard := tgbotapi.NewInlineKeyboardMarkup()
	for _, task := range tasks {
		keyboard.InlineKeyboard = append(keyboard.InlineKeyboard, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(task.TaskName, fmt.Sprintf("%d", task.ID)),
		))
	}
	msg.ReplyMarkup = keyboard
	sentMessage, _ := app.Bot.Send(msg)
	app.LastChatMessage[update.CallbackQuery.From.ID] = &sentMessage
	app.UserPlace[update.CallbackQuery.From.ID] = START_POMO1
}

func (app *App) StartPOMO1(update tgbotapi.Update) {
	id, _ := strconv.Atoi(update.CallbackData())
	app.TaskToComplete[update.CallbackQuery.From.ID] = uint(id)
	msg := tgbotapi.NewMessage(app.Chatids[update.CallbackQuery.From.ID], "Please submit a duration for this pomo:")
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("15 minutes", "15"),
			tgbotapi.NewInlineKeyboardButtonData("20 minutes", "20"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("25 minutes", "25"),
			tgbotapi.NewInlineKeyboardButtonData("30 minutes", "30"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("35 minutes", "35"),
			tgbotapi.NewInlineKeyboardButtonData("40 minutes", "40"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("45 minutes", "45"),
			tgbotapi.NewInlineKeyboardButtonData("50 minutes", "50"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("55 minutes", "55"),
			tgbotapi.NewInlineKeyboardButtonData("60 minutes", "60"),
		),
	)
	msg.ReplyMarkup = keyboard
	sentMessage, _ := app.Bot.Send(msg)
	app.LastChatMessage[update.CallbackQuery.From.ID] = &sentMessage
	app.UserPlace[update.CallbackQuery.From.ID] = START_POMO2
}

func (app *App) StartTimer(update tgbotapi.Update) {
	timer := app.PomoTimer[update.CallbackQuery.From.ID]
	timer.Starttime = time.Now()
	app.PomoTimer[update.CallbackQuery.From.ID] = timer
	select {
	case <-time.After(timer.Duration):
		task := &POMOtask{}
		app.DB.Where("ID = ?", app.TaskToComplete[update.CallbackQuery.From.ID]).First(&task)
		task.PomosDone++
		if msg, ok := app.LastChatMessage[update.CallbackQuery.From.ID]; ok {
			deleteconfig := tgbotapi.DeleteMessageConfig{
				ChatID:    msg.Chat.ID,
				MessageID: msg.MessageID,
			}
			app.Bot.Request(deleteconfig)
		}
		app.DB.Save(&task)
		msg := tgbotapi.NewMessage(app.Chatids[update.CallbackQuery.From.ID], fmt.Sprintf("Your POMO is completed!!\ntask: %s is now %s", task.TaskName, func(first, second int) string {
			if first == second {
				return "completed ✅"
			}
			return "1 pomo ahead"
		}(task.NeededPomo, task.PomosDone)))
		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("Back to main menu", "start"),
			),
		)
		msg.ReplyMarkup = keyboard
		sentMessage, _ := app.Bot.Send(msg)
		app.LastChatMessage[update.CallbackQuery.From.ID] = &sentMessage
	case <-app.StopChan:
		timer := app.PomoTimer[update.CallbackQuery.From.ID]
		timer.Duration = timer.Duration - time.Since(timer.Starttime)
		app.PomoTimer[update.CallbackQuery.From.ID] = timer
		msg := tgbotapi.NewMessage(app.Chatids[update.CallbackQuery.From.ID], fmt.Sprintf("Timer stoped! remaining time: %d:%d", int(timer.Duration.Minutes()), int(timer.Duration.Seconds())%60))
		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("Start timer", "restart"),
			),
		)
		msg.ReplyMarkup = keyboard
		sentMessage, _ := app.Bot.Send(msg)
		app.LastChatMessage[update.CallbackQuery.From.ID] = &sentMessage
	}
}

func (app *App) StartPOMO2(update tgbotapi.Update) {
	mins, _ := strconv.Atoi(update.CallbackData())
	timer := &Timer{
		Duration: time.Duration(mins*60) * time.Second,
	}
	app.PomoTimer[update.CallbackQuery.From.ID] = timer
	msg := tgbotapi.NewMessage(app.Chatids[update.CallbackQuery.From.ID], fmt.Sprintf("Your POMO of %s minutes Started!\n I will notify you when it ends", update.CallbackData()))
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Show remaining time", "remaining"),
			tgbotapi.NewInlineKeyboardButtonData("Stop timer", "stop"),
		),
	)
	msg.ReplyMarkup = keyboard
	sentMessage, _ := app.Bot.Send(msg)
	app.LastChatMessage[update.CallbackQuery.From.ID] = &sentMessage
	go app.StartTimer(update)
}

func (app *App) ShowErr(update tgbotapi.Update) {
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Please use one of the buttons to interact with bot")
	sentMessage, _ := app.Bot.Send(msg)
	app.LastChatMessage[update.Message.From.ID] = &sentMessage
}

func (app *App) ShowRemainingTime(update tgbotapi.Update) {
	remaining := app.PomoTimer[update.CallbackQuery.From.ID].Duration - time.Since(app.PomoTimer[update.CallbackQuery.From.ID].Starttime)
	msg := tgbotapi.NewMessage(app.Chatids[update.CallbackQuery.From.ID], fmt.Sprintf("%02d:%02d remaining!", int(remaining.Minutes()), int(remaining.Seconds())%60))
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Show remaining time", "remaining"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Stop timer", "stop"),
		),
	)
	msg.ReplyMarkup = keyboard
	sentMessage, _ := app.Bot.Send(msg)
	app.LastChatMessage[update.CallbackQuery.From.ID] = &sentMessage
}

func (app *App) Restart(update tgbotapi.Update) {
	msg := tgbotapi.NewMessage(app.Chatids[update.CallbackQuery.From.ID], "Your POMO restarted!\n I will notify you when it ends")
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Show remaining time", "remaining"),
			tgbotapi.NewInlineKeyboardButtonData("Stop timer", "stop"),
		),
	)
	msg.ReplyMarkup = keyboard
	sentMessage, _ := app.Bot.Send(msg)
	app.LastChatMessage[update.CallbackQuery.From.ID] = &sentMessage
	go app.StartTimer(update)
}

func GetDB() *gorm.DB {
	database := os.Getenv("BOT_DATABASE")
	port := os.Getenv("DB_PORT")
	host := os.Getenv("DB_HOST")
	user := os.Getenv("DB_USER")
	password := os.Getenv("DB_PASSWORD")
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable", host, port, user, password, database)
	connecttion, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		panic(err)
	}
	return connecttion
}

func main() {
	godotenv.Load(".env")
	botapi := os.Getenv("TGBOT_API")
	b, _ := tgbotapi.NewBotAPI(botapi)
	app := &App{
		Bot:             b,
		DB:              GetDB(),
		UserPlace:       make(map[int64]int),
		LastChatMessage: make(map[int64]*tgbotapi.Message),
		TaskToComplete:  make(map[int64]uint),
		AddingTask:      make(map[int64]POMOtask),
		Chatids:         make(map[int64]int64),
		PomoTimer:       make(map[int64]*Timer),
		StopChan:        make(chan bool),
	}
	app.DB.AutoMigrate(&POMOtask{})
	app.Bot.Debug = true
	updateconfig := tgbotapi.NewUpdate(0)
	updateconfig.Timeout = 60
	updates := app.Bot.GetUpdatesChan(updateconfig)
	for update := range updates {
		if update.CallbackQuery != nil {
			if msg, ok := app.LastChatMessage[update.CallbackQuery.From.ID]; ok {
				deleteconfig := tgbotapi.DeleteMessageConfig{
					ChatID:    msg.Chat.ID,
					MessageID: msg.MessageID,
				}
				app.Bot.Request(deleteconfig)
			}
			switch update.CallbackData() {
			case "start":
				app.MainMenu(update)
				continue
			case "start pomo":
				app.StartPOMO(update)
				continue
			case "add task":
				app.AddTaskMesssage(update)
				continue
			case "tasks list":
				app.ShowTasksList(update)
				continue
			case "completed tasks":
				app.ShowCompleted(update)
				continue
			case "remaining":
				app.ShowRemainingTime(update)
				continue
			case "stop":
				app.StopChan <- true
				continue
			case "restart":
				app.Restart(update)
				continue
			}
			if app.UserPlace[update.CallbackQuery.From.ID] == START_POMO1 {
				app.StartPOMO1(update)
			} else if app.UserPlace[update.CallbackQuery.From.ID] == START_POMO2 {
				app.StartPOMO2(update)
			} else {
				msg := tgbotapi.NewMessage(app.Chatids[update.CallbackQuery.From.ID], "Unknown command or message")
				keyboard := tgbotapi.NewInlineKeyboardMarkup(
					tgbotapi.NewInlineKeyboardRow(
						tgbotapi.NewInlineKeyboardButtonData("Back to main menu", "start"),
					),
				)
				msg.ReplyMarkup = keyboard
				sentMessage, _ := app.Bot.Send(msg)
				app.LastChatMessage[update.CallbackQuery.From.ID] = &sentMessage
			}
		} else if update.Message != nil {
			if msg, ok := app.LastChatMessage[update.Message.From.ID]; ok {
				deleteconfig := tgbotapi.DeleteMessageConfig{
					ChatID:    msg.Chat.ID,
					MessageID: msg.MessageID,
				}
				app.Bot.Request(deleteconfig)
				deleteconfig.MessageID = update.Message.MessageID
				app.Bot.Request(deleteconfig)
			}
			if update.Message.IsCommand() {
				switch update.Message.Command() {
				case "start":
					app.ShowStartMessage(update)
					deleteconfig := tgbotapi.DeleteMessageConfig{
						ChatID:    update.Message.Chat.ID,
						MessageID: update.Message.MessageID,
					}
					app.Bot.Request(deleteconfig)
					app.Chatids[update.Message.From.ID] = update.Message.Chat.ID
				case "help":
				}
			} else if update.Message.Text != "" {
				switch app.UserPlace[update.Message.From.ID] {
				case ADD_TASK_PART1:
					app.AddTaskp1(update)
				case ADD_TASK_PART2:
					app.AddTaskp2(update)
				case IDLE:
					app.ShowErr(update)
				}
			}
		}
	}
}
