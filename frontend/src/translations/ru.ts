export default {
  connection: {
    createNewChat: "Создать новый чат",
    joinChat: "Присоединиться к чату",
    nodeUrl: "АДРЕС СЕРВЕРА:",
    username: "ИМЯ ПОЛЬЗОВАТЕЛЯ:",
    usernamePlaceholder: "Введите ваше имя",
    enterChatId: "Введите ID чата для подключения",
    password: "ПАРОЛЬ (необязательно):",
    passwordPlaceholder: "Введите пароль комнаты",
    or: "или",
    connect: "Подключиться",
    disconnect: "Отключиться",
  },
  chat: {
    room: "Комната",
    online: "онлайн",
    typeMessage: "Введите сообщение...",
    send: "Отправить",
    noMessages: "Начните общение",
    encrypted: "Сквозное шифрование",
    userJoined: "присоединился к чату",
    userLeft: "покинул чат",
  },
  errors: {
    connectionFailed: "Не удалось подключиться",
    sendFailed: "Не удалось отправить сообщение",
    invalidPassword: "Неверный пароль",
  },
  security: {
    keyMismatch:
      "Предупреждение безопасности: Несоответствие отпечатка ключа для",
    expected: "Ожидалось",
    received: "Получено",
  },
} as const;
