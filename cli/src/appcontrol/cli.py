import click

from appcontrol.restart import restart
from appcontrol.status import status
from appcontrol.config import config


@click.group()
def cli():
    pass


cli.add_command(status)
cli.add_command(restart)
cli.add_command(config)


if __name__ == "__main__":
    cli()
