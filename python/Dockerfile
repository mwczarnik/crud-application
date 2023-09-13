FROM python:3.11.5-slim-bookworm

WORKDIR /app

COPY . /app

RUN apt update && apt install gcc libev-dev -y && rm -rf /var/lib/apt/lists/*
RUN pip install --trusted-host pypi.python.org Flask pymongo marshmallow_dataclass bjoern

EXPOSE 8080

CMD ["python3", "main.py"]