# Telegram Nudge Bot

Sends you a casual, randomly-timed "lead-style" accountability message a few times a day — generated fresh each time by a free LLM, delivered via Telegram.

## 1. Create your Telegram bot
1. Open Telegram, message **@BotFather**.
2. Send `/newbot`, follow the prompts, name it whatever you want.
3. BotFather gives you a token like `123456789:ABCdefGhIJKlmNoPQRstuVWXyz`. Save it.
4. Send your new bot **any message** first (e.g. "hi") so it's allowed to message you back.
5. Get your chat ID: open this URL in your browser (with your real token):
   `https://api.telegram.org/bot<YOUR_TOKEN>/getUpdates`
   Find `"chat":{"id":123456789,...}` in the response — that number is your `TELEGRAM_CHAT_ID`.

## 2. Get a free Groq API key
1. Go to https://console.groq.com, sign up (free).
2. Create an API key under API Keys. Save it.

## 3. Put this code in a GitHub repo
1. Create a new **private** repo on GitHub.
2. Push these files (`main.go`, `.github/workflows/nudge.yml`, this README) to it.

## 4. Add your secrets to the repo
In the repo: **Settings → Secrets and variables → Actions → New repository secret**. Add three:
- `GROQ_API_KEY`
- `TELEGRAM_BOT_TOKEN`
- `TELEGRAM_CHAT_ID`

## 5. Test it
Go to the **Actions** tab → "Send Nudge" workflow → **Run workflow** (this is the `workflow_dispatch` trigger) to fire it manually and confirm a message lands in Telegram within ~45 minutes (it sleeps a random amount first — see `main.go`).

## 6. It's live
Once secrets are set, the cron schedule in `nudge.yml` runs it automatically — no laptop required, it runs in GitHub's cloud. Default is 3 rough windows a day (morning/afternoon/evening); edit the `cron` lines to change timing. Cron times are in **UTC** — adjust for your timezone.

## Changing the topic
Just text your bot on Telegram, anytime:

```
new-topic: leetcode, 2 questions a day
```

The bot checks for new messages at the start of every run. If it sees one starting with `new-topic:`, it switches focus to whatever follows and keeps using that topic for every future nudge — until you send another `new-topic:` message. The current topic (and Telegram's message offset, so it doesn't re-read old messages) is stored in `state.json` in the repo, which the workflow commits back after each run — that's why `permissions: contents: write` is set in `nudge.yml`.

You don't need to wait for a scheduled run to see it take effect on the *next* real nudge — but if you want to confirm it registered immediately, trigger a manual run from the Actions tab (`workflow_dispatch`) after sending the message.

## Notes
- Everything here is free: GitHub Actions free tier, Groq's free tier, Telegram bots.
- To change the "lead" persona or message styles, edit `promptTemplate` in `main.go`.
- If a run fails, check the Actions tab — it'll show the error (usually a missing/wrong secret).
- `state.json` starts with a sensible default topic already filled in — no setup needed there, just push it as-is.
