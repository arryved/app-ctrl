import click

from appcontrol.restart import restart
from appcontrol.status import status


@click.group()
def cli():
    pass


cli.add_command(status)
cli.add_command(restart)


if __name__ == "__main__":
    cli()
