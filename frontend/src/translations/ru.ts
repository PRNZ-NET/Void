export default {
    connection: {
        createNewChat: "Создать новый чат",
        joinChat: "Присоединиться к чату",
        nodeUrl: "АДРЕС СЕРВЕРА:",
        username: "ИМЯ ПОЛЬЗОВАТЕЛЯ:",
        usernamePlaceholder: "Введите ваше имя",
        enterChatId: "Введите ID чата для подключения",
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
    },
    errors: {
        connectionFailed: "Не удалось подключиться",
        sendFailed: "Не удалось отправить сообщение",
    },
    security: {
        keyMismatch: "Предупреждение безопасности: Несоответствие отпечатка ключа для",
        expected: "Ожидалось",
        received: "Получено",
    }
} as const;

