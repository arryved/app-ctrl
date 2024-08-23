import base64
import click
import click_spinner
import datetime
import json
import math
import pwinput
import pytz
import requests
import sys
import warnings

from ansitable import ANSITable, Column

from appcontrol.common import constants
from appcontrol.auth import token


warnings.filterwarnings("ignore")


@click.group()
def secrets():
    pass


def get_encoded_secret(file):
    secret = ""
    if file:
        if file == "-":
            secret = sys.stdin.buffer.read()
        else:
            with open(file, "rb") as fh:
                secret = fh.read()
    else:
        secret = pwinput.pwinput(prompt='Enter secret: ', mask='•').encode('utf-8')
    return base64.b64encode(secret).decode('utf-8')


@click.command()
@click.option('-e', '--environment', required=True)
@click.option('-n', '--name', required=True)
@click.option('-g', '--group', required=True)
@click.option('-f', '--file', required=False)
def create(environment, name, group, file):
    value = get_encoded_secret(file)

    action = "secrets"
    api_host = constants["api_hosts_by_env"][environment]
    url = (f"{api_host}/{action}/{environment}")
    click.echo(click.style(f"Connecting to {url} ...", fg="green"), err=True)
    with click_spinner.spinner():
        body = {
                "id": name,
                "ownerGroup": group,
                "value": value,
        }
        id_token = token().get("id_token")
        headers = {"Authorization": f"Bearer {id_token}"}
        response = requests.post(url, json=body, headers=headers, verify=True)

    status_code = math.floor(response.status_code / 100)
    if status_code == 5:
        error = json.loads(response.text).get("error", str(response.text))
        click.echo(click.style(f"Server experienced an error: {error}", fg="red"), err=True)
        exit()

    if status_code == 4:
        error = json.loads(response.text).get("error", str(response.text))
        click.echo(click.style(f"Request error: {error}", fg="yellow"), err=True)

    if status_code == 2:
        click.echo(click.style(f"Secret successfully created in env={environment}", fg="green"), err=True)
        click.echo(click.style(f"{name}"), err=False)


def is_binary(data):
    # Check if the data contains any non-printable or control characters
    # Excluding common whitespace like space, newline, carriage return, tab
    text_chars = bytes({7, 8, 9, 10, 12, 13, 27} | set(range(0x20, 0x7F)))

    # Check if any byte in the data is not in the set of text characters
    return any(byte not in text_chars for byte in data)


@click.command()
@click.option('-e', '--environment', required=True)
@click.option('-n', '--name', required=True)
@click.option('-f', '--file', required=False)
def get(environment, name, file):
    action = "secrets"
    api_host = constants["api_hosts_by_env"][environment]
    url = (f"{api_host}/{action}/{environment}/{name}")

    click.echo(click.style(f"Connecting to {url} ...", fg="green"), err=True)
    with click_spinner.spinner():
        id_token = token().get("id_token")
        headers = {"Authorization": f"Bearer {id_token}"}
        response = requests.get(url, headers=headers, verify=True)

    status_code = math.floor(response.status_code / 100)
    if status_code == 5:
        error = json.loads(response.text).get("error", str(response.text))
        click.echo(click.style(f"Server experienced an error: {error}", fg="red"), err=True)
        exit(1)

    if status_code == 4:
        error = json.loads(response.text).get("error", str(response.text))
        click.echo(click.style(f"Request error: {error}", fg="yellow"), err=True)

    if status_code == 2:
        click.echo(click.style(f"Secret urn:arryved:secret:{name} successfully fetched from env={environment}", fg="cyan"), err=True)
        decoded = base64.b64decode(response.text)
        if file:
            with open(file, "wb") as fh:
                fh.write(decoded)
                click.echo(click.style(f"Binary data written to {file}", fg="green"), err=True)
                exit(1)
        try:
            if not is_binary(decoded):
                decoded_string = decoded.decode('utf-8')
                print(decoded_string)
                exit(0)
            else:
                click.echo(click.style("Refusing to print binary (non-printable); use --file to dump to output", fg="yellow"), err=True)
                exit(2)
        except UnicodeDecodeError:
            click.echo(click.style("Refusing to print binary (non-unicode); use --file to dump to output", fg="yellow"), err=True)
            exit(2)
        click.echo(click.style(f"{response.text}"), err=False)
        exit(0)


def ns_to_human_time(ns_epoch, utc=False):
    seconds = ns_epoch / 1e9
    dt = datetime.datetime.utcfromtimestamp(seconds)
    if utc:
        return dt.strftime('%Y-%m-%d %H:%M:%SZ')
    local_tz = datetime.datetime.now().astimezone().tzinfo
    dt_local = dt.replace(tzinfo=pytz.utc).astimezone(local_tz)
    return dt_local.strftime('%Y-%m-%d %H:%M:%S')


def print_secrets_table(results, utc):
    table = ANSITable(
        Column("Name", headstyle="bold"),
        Column("Owner Group", headstyle="bold"),
        Column("Owner User", headstyle="bold"),
        Column("Created", headstyle="bold"),
        border="thin"
    )

    for result in results:
        name = result["urn"].split(":")[-1]
        group = result["ownerGroup"]
        user = result["ownerUser"]
        created = ns_to_human_time(result["createdEpochNs"], utc=utc)
        table.row(name, group, user, created)

    table.print()


@click.command()
@click.option('-e', '--environment', required=True)
@click.option('-u', '--utc', default=False, is_flag=True, help='show time in UTC instead of local tz')
def list(environment, utc):
    action = "secrets"
    api_host = constants["api_hosts_by_env"][environment]
    url = (f"{api_host}/{action}/{environment}")

    click.echo(click.style(f"Connecting to {url} ...", fg="green"), err=True)
    with click_spinner.spinner():
        id_token = token().get("id_token")
        headers = {"Authorization": f"Bearer {id_token}"}
        response = requests.get(url, headers=headers, verify=True)

    status_code = math.floor(response.status_code / 100)
    if status_code == 5:
        error = json.loads(response.text).get("error", str(response.text))
        click.echo(click.style(f"Server experienced an error: {error}", fg="red"), err=True)
        exit(1)

    if status_code == 4:
        error = json.loads(response.text).get("error", str(response.text))
        click.echo(click.style(f"Request error: {error}", fg="yellow"), err=True)
        exit(2)

    if status_code == 2:
        print_secrets_table(json.loads(response.text), utc)
        exit(0)


@click.command()
@click.option('-e', '--environment', required=True)
@click.option('-n', '--name', required=True)
@click.option('-f', '--file', required=False)
def update(environment, name, file):
    value = get_encoded_secret(file)

    action = "secrets"
    api_host = constants["api_hosts_by_env"][environment]
    url = (f"{api_host}/{action}/{environment}/{name}")
    click.echo(click.style(f"Connecting to {url} ...", fg="green"), err=True)
    with click_spinner.spinner():
        body = {
                "value": value,
        }
        id_token = token().get("id_token")
        headers = {"Authorization": f"Bearer {id_token}"}
        response = requests.patch(url, json=body, headers=headers, verify=True)

    status_code = math.floor(response.status_code / 100)
    if status_code == 5:
        error = json.loads(response.text).get("error", str(response.text))
        click.echo(click.style(f"Server experienced an error: {error}", fg="red"), err=True)
        exit()

    if status_code == 4:
        error = json.loads(response.text).get("error", str(response.text))
        click.echo(click.style(f"Request error: {error}", fg="yellow"), err=True)

    if status_code == 2:
        click.echo(click.style(f"Secret successfully created in env={environment}", fg="green"), err=True)


@click.command()
@click.option('-e', '--environment', required=True)
@click.option('-n', '--name', required=True)
def delete(environment, name):
    action = "secrets"
    api_host = constants["api_hosts_by_env"][environment]
    url = (f"{api_host}/{action}/{environment}/{name}")

    click.echo(click.style(f"Connecting to {url} ...", fg="green"), err=True)
    with click_spinner.spinner():
        id_token = token().get("id_token")
        headers = {"Authorization": f"Bearer {id_token}"}
        response = requests.delete(url, headers=headers, verify=True)

    status_code = math.floor(response.status_code / 100)
    if status_code == 5:
        error = json.loads(response.text).get("error", str(response.text))
        click.echo(click.style(f"Server experienced an error: {error}", fg="red"), err=True)
        exit(1)

    if status_code == 4:
        error = json.loads(response.text).get("error", str(response.text))
        click.echo(click.style(f"Request error: {error}", fg="yellow"), err=True)
        exit(2)

    if status_code == 2:
        click.echo(click.style(f"Secret urn:arryved:secret:{name} successfully deleted from env={environment}", fg="green"), err=True)
        exit(0)


secrets.add_command(create)
secrets.add_command(get)
secrets.add_command(list)
secrets.add_command(update)
secrets.add_command(delete)
