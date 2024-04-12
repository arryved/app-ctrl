import click

from appcontrol.restart import restart
from appcontrol.status import status
from appcontrol.config import config
from appcontrol.deploy import deploy


@click.group()
def cli():
    pass


cli.add_command(status)
cli.add_command(restart)
cli.add_command(config)
cli.add_command(deploy)


if __name__ == "__main__":
    cli()
