FROM python:3.6-slim

WORKDIR /app

COPY . /app

RUN pip install --trusted-host pypi.python.org Flask pymongo  marshmallow_dataclass waitress redis 

EXPOSE 8080

ENV FLASK_APP=main.py
ENV FLASK_RUN_HOST=0.0.0.0
ENV FLASK_RUN_PORT=8080


CMD ["flask", "run"]