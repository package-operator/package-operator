import requests
import os
import datetime


slack_webhook_url = os.getenv("SLACK_WEBHOOK_URL")
if slack_webhook_url is None:
    raise RuntimeError("SLACK_WEBHOOK_URL environment variable not set")

rotation = os.getenv("RELEASE_ROTATION")
if rotation is None:
    raise RuntimeError("RELEASE_ROTATION environment variable not set")

rotation = rotation.split(',')
week = datetime.datetime.now().isocalendar()[1]
data = {
    "user_id": rotation[week % len(rotation)]
}

response = requests.post(slack_webhook_url, json=data)
if response.status_code != 200:
    raise ValueError(f"Request to Slack returned error {response.status_code}, {response.text}")

print("Success!")
