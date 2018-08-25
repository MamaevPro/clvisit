package main

import (
	"net"
	"strings"
	"os"
	"encoding/json"
	"net/url"
	"fmt"
	"net/http"
	"strconv"
)



func processDeauth(message Message, conn *net.Conn) {
	logAdd(MESS_INFO, "Пришел отказ на авторизацию")

	if myClient.Conn != nil {
		(*myClient.Conn).Close()
	}
}

func processAuth(message Message, conn *net.Conn) {
	logAdd(MESS_INFO, "Пришел ответ на авторизацию")
	if len(message.Messages) != 2 {
		logAdd(MESS_ERROR, "Не правильное кол-во полей")
	}

	myClient.Pid = message.Messages[0]
	myClient.Salt = message.Messages[1]

	if len(options.Pass) == 0 {
		logAdd(MESS_INFO, "Сгенерировали новый пароль")

		if DEFAULT_NUMBER_PASSWORD {
			options.Pass = encXOR(randomNumber(LENGTH_PASS), myClient.Pid)
		} else {
			options.Pass = encXOR(randomString(LENGTH_PASS), myClient.Pid)
		}

		saveOptions()
	}

	sendMessageToLocalCons(TMESS_LOCAL_INFO, myClient.Pid, getPass(), myClient.Version,
			options.HttpServerClientType + "://" + options.HttpServerClientAdr + ":" + options.HttpServerClientPort,
			options.HttpServerType + "://" + options.HttpServerAdr + ":" + options.HttpServerPort,
			options.ProfileLogin, options.ProfilePass)
}

func processLogin(message Message, conn *net.Conn) {
	logAdd(MESS_INFO, "Пришел ответ на вход в учетку")

	sendMessageToLocalCons(TMESS_LOCAL_LOGIN)
}

func processNotification(message Message, conn *net.Conn) {
	logAdd(MESS_INFO, "Пришло уведомление")
	if len(message.Messages) != 1 {
		logAdd(MESS_ERROR, "Не правильное кол-во полей")
	}

	sendMessageToLocalCons(TMESS_LOCAL_NOTIFICATION, message.Messages[0])
}

func processConnect(message Message, conn *net.Conn) {
	logAdd(MESS_INFO, "Пришел запрос на подключение")
	if len(message.Messages) <= 5 {
		logAdd(MESS_ERROR, "Не правильное кол-во полей")
	}

	digest := message.Messages[0]
	salt := message.Messages[1]
	code := message.Messages[2]
	tconn := message.Messages[3]
	ctype := message.Messages[4]

	if getSHA256(getPass() + salt) != digest  && ctype == "server" {
		logAdd(MESS_ERROR, "Не верный пароль")
		sendMessage(TMESS_NOTIFICATION, message.Messages[5], "Не прошлил аутентификацию!")
		sendMessage(TMESS_DISCONNECT, code)
		return
	}

	if flagReinstallVnc || options.ActiveVncId == -1 {
		logAdd(MESS_ERROR, "Не готов VNC")
		sendMessage(TMESS_NOTIFICATION, message.Messages[5], "Не готов VNC!")
		sendMessage(TMESS_DISCONNECT, code)
		return
	}

	if tconn == "simple" {
		logAdd(MESS_INFO, "Запускаем \"простой\" тип подключения")
		if ctype == "server" {
			go convisit(options.DataServerAdr + ":" + options.DataServerPort, options.LocalAdrVNC + ":" + arrayVnc[options.ActiveVncId].PortServerVNC, code, false, 1); //тот кто передает трансляцию
		} else {
			go convisit(options.DataServerAdr + ":" + options.DataServerPort, options.LocalAdrVNC + ":" + options.PortClientVNC, code, false, 2); //тот кто получает трансляцию
		}
	} else {
		logAdd(MESS_INFO, "Не известный тип подключения")
		sendMessage(TMESS_NOTIFICATION, message.Messages[5], "Не известный тип подключения!")
	}
}

func processDisconnect(message Message, conn *net.Conn) {
	logAdd(MESS_INFO, "Пришел запрос на отключение")
	if len(message.Messages) <= 1 {
		logAdd(MESS_ERROR, "Не правильное кол-во полей")
	}
}

func processContacts(message Message, conn *net.Conn) {
	logAdd(MESS_INFO, "Пришли контакты")
	dec, err := url.PathUnescape(message.Messages[0])
	if err == nil {
		contact := Contact{}
		err = json.Unmarshal([]byte(dec), &contact)
		if dec != "null" {
			if err == nil {
				myClient.Profile.Contacts = &contact
				b, err := json.Marshal(contact)
				if err == nil {
					sendMessageToLocalCons(TMESS_LOCAL_CONTACTS, url.PathEscape(string(b)))
				}
			}else{
				fmt.Println(err)
			}
		}
	}
}

func processContact(message Message, conn *net.Conn) {
	logAdd(MESS_INFO, "Пришло изменение контакта")

	sendMessageToLocalCons(TMESS_LOCAL_CONTACT, message.Messages...)
}

func processStatus(message Message, conn *net.Conn) {
	logAdd(MESS_INFO, "Пришел статус контакта")

	sendMessageToLocalCons(TMESS_LOCAL_STATUS, message.Messages...)
}

func processInfoContact(message Message, conn *net.Conn) {
	logAdd(MESS_INFO, "Пришел запрос на информацию")

	if getSHA256(getPass() + message.Messages[2]) == message.Messages[1] {
		//todo добавить много всякой инфы
		hostname, _ := os.Hostname()

		//uptime := gosigar.Uptime{}
		//uptime.Get()


		sendMessage(TMESS_INFO_ANSWER, message.Messages[0], fmt.Sprint(options.ActiveVncId), hostname, GetInfoOS(), REVISIT_VERSION )
		return
	}

	logAdd(MESS_ERROR, "Не правильные контрольные данные")
}

func processInfoAnswer(message Message, conn *net.Conn) {
	logAdd(MESS_INFO, "Пришел ответ запроса на информацию")

	sendMessageToLocalCons(TMESS_LOCAL_INFO_ANSWER, message.Messages...)
}

func processManage(message Message, conn *net.Conn) {
	logAdd(MESS_INFO, "Пришел запрос на управление")

	//Message[0] who called(pid)
	//Message[1] digest
	//Message[2] salt
	//Message[3] act

	if getSHA256(getPass() + message.Messages[2]) == message.Messages[1] {

		if message.Messages[3] == "revnc" {
			i, err := strconv.Atoi(message.Messages[4])
			if err == nil {
				go processVNC(i)
				return
			}
			logAdd(MESS_ERROR, "Не получилось обновить VNC")
			return
		} else if message.Messages[3] == "update" {
			updateMe()
			return
		} else if message.Messages[3] == "reload" {
			reloadMe()
			return
		} else if message.Messages[3] == "restart" {
			restartSystem()
			return
		}

		logAdd(MESS_ERROR, "Что-то пошло не так")
		return
	}

	logAdd(MESS_ERROR, "Не правильные контрольные данные")
}

func processPing(message Message, conn *net.Conn) {
	//logAdd(MESS_INFO, "Пришел пинг")
}



func processLocalTest(message Message, conn *net.Conn) {
	logAdd(MESS_INFO, "Пришел локальный тест")

	sendMessageToSocket(conn, message.TMessage, "test")
}

func processLocalInfo(message Message, conn *net.Conn) {
	logAdd(MESS_INFO, "Пришел локальный запрос на реквизиты")

	if connections.count > 0 {
		sendMessageToSocket(conn, message.TMessage, "XX:XX:XX:XX", "*****", myClient.Version,
			options.HttpServerClientType + "://" + options.HttpServerClientAdr + ":" + options.HttpServerClientPort,
			options.HttpServerType + "://" + options.HttpServerAdr + ":" + options.HttpServerPort,
			options.ProfileLogin, options.ProfilePass)
	} else {
		if len(message.Messages) > 0 {
			options.Pass = encXOR(message.Messages[0], myClient.Pid)
			saveOptions()
		}

		sendMessageToSocket(conn, message.TMessage, myClient.Pid, getPass(), myClient.Version,
			options.HttpServerClientType + "://" + options.HttpServerClientAdr + ":" + options.HttpServerClientPort,
			options.HttpServerType + "://" + options.HttpServerAdr + ":" + options.HttpServerPort,
			options.ProfileLogin, options.ProfilePass)
	}
}

func processLocalConnect(message Message, conn *net.Conn) {
	logAdd(MESS_INFO, "Пришел локальный запрос на подключение")

	sendMessage(TMESS_REQUEST, message.Messages[0], getSHA256(message.Messages[1] + myClient.Salt))
}

func processLocalInfoClient(message Message, conn *net.Conn) {
	logAdd(MESS_INFO, "Пришел локальный запрос о vnc клиенте")

	if options.ActiveVncId != -1 {
		if checkForAdmin() {
			sendMessageToSocket(conn, message.TMessage,
				parentPath + VNC_FOLDER + string(os.PathSeparator) + arrayVnc[options.ActiveVncId].Name + "_" + arrayVnc[options.ActiveVncId].Version + string(os.PathSeparator) + strings.Replace(arrayVnc[options.ActiveVncId].CmdStartClient, "%adr", options.LocalAdrVNC  + ":" + options.PortClientVNC, 1),
				parentPath + VNC_FOLDER + string(os.PathSeparator) + arrayVnc[options.ActiveVncId].Name + "_" + arrayVnc[options.ActiveVncId].Version + string(os.PathSeparator) + arrayVnc[options.ActiveVncId].CmdManageServer )
		} else {
			sendMessageToSocket(conn, message.TMessage,
				parentPath + VNC_FOLDER + string(os.PathSeparator) + arrayVnc[options.ActiveVncId].Name + "_" + arrayVnc[options.ActiveVncId].Version+string(os.PathSeparator) + strings.Replace(arrayVnc[options.ActiveVncId].CmdStartClient, "%adr", options.LocalAdrVNC + ":" + options.PortClientVNC, 1),
				parentPath + VNC_FOLDER + string(os.PathSeparator) + arrayVnc[options.ActiveVncId].Name + "_" + arrayVnc[options.ActiveVncId].Version+string(os.PathSeparator) + arrayVnc[options.ActiveVncId].CmdManageServerUser)
		}
	}
}

func processTerminate(message Message, conn *net.Conn) {
	logAdd(MESS_INFO, "Пришел запрос на завершение и удаление")

	if message.Messages[0] == "1" {
		terminateMe(true)
	} else {
		sendMessageToSocket(conn, message.TMessage)
		terminateMe(false)
	}

}

func processLocalReg(message Message, conn *net.Conn) {
	logAdd(MESS_INFO, "Пришел локальный запрос на регистрацию учетки")

	sendMessage(TMESS_REG, message.Messages[0])
}

func processLocalLogin(message Message, conn *net.Conn) {
	logAdd(MESS_INFO, "Пришел локальный запрос на вход в учетку")

	if len(myClient.Pid) == 0 {
		logAdd(MESS_INFO, "Ещё не готовы к авторизации в профиль")
		return
	}

	if message.Messages[2] == "1" {
		logAdd(MESS_INFO, "Сохраним данные для входа в профиль")
		options.ProfileLogin = message.Messages[0]
		options.ProfilePass = message.Messages[1]
		saveOptions()
	} else {
		logAdd(MESS_INFO, "Удалим данные для входа в профиль")
		options.ProfileLogin = ""
		options.ProfilePass = ""
		saveOptions()
	}
	sendMessage(TMESS_LOGIN, message.Messages[0], getSHA256(message.Messages[1] + myClient.Salt))
}

func processLocalContact(message Message, conn *net.Conn) {
	logAdd(MESS_INFO, "Пришел локальный запрос управления контактом")

	digest := ""
	if len(message.Messages[4]) > 0 {
		digest = getSHA256(message.Messages[4] + myClient.Salt)
	}
	sendMessage(TMESS_CONTACT, message.Messages[0], message.Messages[1], message.Messages[2], message.Messages[3], digest, message.Messages[5] )
}

func processLocalContacts(message Message, conn *net.Conn) {
	logAdd(MESS_INFO, "Пришел локальный запрос на обновления списка контактов")

	sendMessage(TMESS_CONTACTS)
}

func processLocalLogout(message Message, conn *net.Conn) {
	logAdd(MESS_INFO, "Пришел локальный запрос на выход")
	myClient.Profile.Contacts = nil

	options.ProfileLogin = ""
	options.ProfilePass = ""
	saveOptions()

	sendMessage(TMESS_LOGOUT)
}

func processLocalConnectContact(message Message, conn *net.Conn) {
	logAdd(MESS_INFO, "Пришел локальный запрос на подключение к контакту")

	sendMessage(TMESS_CONNECT_CONTACT, message.Messages[0])
}

func processLocalListVNC(message Message, conn *net.Conn) {
	logAdd(MESS_INFO, "Пришел локальный запрос на список VNC")

	resp, err := http.Get(options.HttpServerType + "://" + options.HttpServerAdr + ":" + options.HttpServerPort + "/api?make=listvnc")
	if err != nil {
		logAdd(MESS_ERROR, "Не получилось получить с сервера VNC: " + fmt.Sprint(err))
		return
	}

	var buff []byte
	buff = make([]byte, options.SizeBuff* options.SizeBuff)
	n, err := resp.Body.Read(buff)

	var listVNC []VNC
	err = json.Unmarshal(buff[:n], &listVNC)
	if err != nil {
		logAdd(MESS_ERROR, "Не получилось получить с сервера VNC: " + fmt.Sprint(err))
		return
	}

	for _, x := range listVNC {
		sendMessageToSocket(conn, TMESS_LOCAL_LISTVNC, x.Name + " " + x.Version, x.Link)
	}

}

func processLocalInfoContact(message Message, conn *net.Conn) {
	logAdd(MESS_INFO, "Пришел локальный запрос на информацию о контакте")

	sendMessage(TMESS_INFO_CONTACT, message.Messages[0])
}

func processLocalManage(message Message, conn *net.Conn) {
	logAdd(MESS_INFO, "Пришел локальный запрос на управление")

	sendMessage(TMESS_MANAGE, message.Messages...)
}

func processLocalSave(message Message, conn *net.Conn) {
	logAdd(MESS_INFO, "Пришел локальный запрос на сохранение опций")

	saveOptions()
}

func processLocalOptionClear(message Message, conn *net.Conn) {
	logAdd(MESS_INFO, "Пришел локальный запрос на восстановление дефолтных опций")

	defaultOptions()
	processVNC(0)
}

func processLocalMyManage(message Message, conn *net.Conn) {
	logAdd(MESS_INFO, "Пришел локальный запрос на своё управление")

	if message.Messages[0] == "revnc" {
		i, err := strconv.Atoi(message.Messages[1])
		if err == nil {
			go processVNC(i)
			return
		}
		logAdd(MESS_ERROR, "Не получилось обновить VNC")
		return
	} else if message.Messages[0] == "update" {
		updateMe()
		return
	} else if message.Messages[0] == "reload" {
		reloadMe()
		return
	} else if message.Messages[0] == "restart" {
		restartSystem()
		return
	}
}

func processLocalMyInfo(message Message, conn *net.Conn) {
	logAdd(MESS_INFO, "Пришел локальный запрос на свою информацию")

	hostname, _ := os.Hostname()
	sendMessageToLocalCons(TMESS_LOCAL_INFO_ANSWER, "", fmt.Sprint(options.ActiveVncId), hostname, GetInfoOS(), REVISIT_VERSION )
}

func processLocalContactReverse(message Message, conn *net.Conn) {
	logAdd(MESS_INFO, "Пришел локальный запрос на добавление в чужой профиль")

	hostname, _ := os.Hostname()
	sendMessage(TMESS_CONTACT_REVERSE, message.Messages[0], getSHA256(message.Messages[1] + myClient.Salt), hostname)
}

func processLocalOptionsUI(message Message, conn *net.Conn) {
	logAdd(MESS_INFO, "Пришел запрос на работу с опциями UI")

	if len(message.Messages) == 0 {
		sendMessageToSocket(conn, message.TMessage, options.OptionsUI)
	} else {
		options.OptionsUI = message.Messages[0]
		saveOptions()
	}
}