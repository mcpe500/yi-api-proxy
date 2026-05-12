(function () {
    const messagesEl = document.getElementById("messages");
    const inputEl = document.getElementById("msg-input");
    const sendBtn = document.getElementById("send-btn");
    const apiKeyEl = document.getElementById("api-key-input");
    const modelEl = document.getElementById("model-select");
    const streamEl = document.getElementById("stream-toggle");
    const tokenBar = document.getElementById("token-bar");

    let conversation = [];
    let sending = false;

    function addMsg(role, text) {
        const div = document.createElement("div");
        div.className = "msg msg-" + role;
        div.textContent = text;
        messagesEl.appendChild(div);
        messagesEl.scrollTop = messagesEl.scrollHeight;
        return div;
    }

    function updateTokens(u) {
        if (!u) return;
        document.getElementById("tok-prompt").textContent = u.prompt_tokens || 0;
        document.getElementById("tok-completion").textContent = u.completion_tokens || 0;
        document.getElementById("tok-total").textContent = u.total_tokens || 0;
        tokenBar.classList.remove("hidden");
    }

    async function send() {
        const apiKey = apiKeyEl.value.trim();
        const model = modelEl.value;
        const stream = streamEl.checked;
        const text = inputEl.value.trim();

        if (!apiKey) { addMsg("error", "Please enter an API key"); return; }
        if (!text || sending) return;

        sending = true;
        sendBtn.disabled = true;
        inputEl.value = "";

        addMsg("user", text);
        conversation.push({ role: "user", content: text });

        const typing = document.createElement("div");
        typing.className = "msg msg-assistant msg-typing";
        typing.textContent = "Thinking...";
        messagesEl.appendChild(typing);
        messagesEl.scrollTop = messagesEl.scrollHeight;

        try {
            const res = await fetch("/v1/chat/completions", {
                method: "POST",
                headers: {
                    "Content-Type": "application/json",
                    "Authorization": "Bearer " + apiKey,
                },
                body: JSON.stringify({
                    model: model,
                    messages: conversation,
                    stream: stream,
                }),
            });

            typing.remove();

            if (!res.ok) {
                const err = await res.json().catch(() => ({ error: { message: "HTTP " + res.status } }));
                throw new Error(err.error?.message || "HTTP " + res.status);
            }

            if (stream) {
                await handleSSE(res);
            } else {
                const data = await res.json();
                const reply = data.choices?.[0]?.message?.content || "";
                addMsg("assistant", reply);
                conversation.push({ role: "assistant", content: reply });
                if (data.usage) updateTokens(data.usage);
            }
        } catch (e) {
            typing.remove();
            addMsg("error", e.message);
            conversation.pop();
        }

        sending = false;
        sendBtn.disabled = false;
        inputEl.focus();
    }

    async function handleSSE(res) {
        const assistantDiv = document.createElement("div");
        assistantDiv.className = "msg msg-assistant";
        messagesEl.appendChild(assistantDiv);

        let fullText = "";
        const reader = res.body.getReader();
        const decoder = new TextDecoder();
        let buffer = "";

        while (true) {
            const { done, value } = await reader.read();
            if (done) break;

            buffer += decoder.decode(value, { stream: true });
            const lines = buffer.split("\n");
            buffer = lines.pop() || "";

            for (const line of lines) {
                if (!line.startsWith("data: ")) continue;
                const data = line.slice(6).trim();
                if (data === "[DONE]") continue;

                try {
                    const json = JSON.parse(data);
                    if (json.error) {
                        addMsg("error", json.error.message || JSON.stringify(json.error));
                        continue;
                    }
                    const token = json.choices?.[0]?.delta?.content;
                    if (token) {
                        fullText += token;
                        assistantDiv.textContent = fullText;
                        messagesEl.scrollTop = messagesEl.scrollHeight;
                    }
                    if (json.usage) updateTokens(json.usage);
                } catch {}
            }
        }

        if (fullText) {
            conversation.push({ role: "assistant", content: fullText });
        } else {
            assistantDiv.remove();
        }
    }

    sendBtn.addEventListener("click", send);

    inputEl.addEventListener("keydown", (e) => {
        if (e.key === "Enter" && !e.shiftKey) {
            e.preventDefault();
            send();
        }
    });

    inputEl.addEventListener("input", () => {
        inputEl.style.height = "auto";
        inputEl.style.height = Math.min(inputEl.scrollHeight, 200) + "px";
    });
})();
