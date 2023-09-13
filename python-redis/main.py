from flask import Flask, request, jsonify
from pymongo import MongoClient
from marshmallow_dataclass import dataclass
from dataclasses import asdict
import bjoern
import json
import redis
import os

app = Flask(__name__)

mongo_uri = os.environ.get('MONGO_URI')
client = MongoClient(mongo_uri)

redis_host = os.environ.get('REDIS_HOST')
redis_client = redis.StrictRedis(
    host=redis_host, port=6379, decode_responses=True)


db = client["crud_db"]
users_collection = db["users"]


def populate_cache(users):
    for user in users:
        del user["_id"]
        redis_client.set(f"user:{user['id']}", json.dumps(user, default=str))


populate_cache(list(users_collection.find()))


@dataclass
class User:
    id: str
    name: str


@app.route("/user", methods=["POST"])
def create_user():
    try:
        user_data = request.json
        user = User.Schema().load(user_data)
        user_json = asdict(user)

        result = users_collection.insert_one(user_json)
        redis_client.set(f"user:{user.id}", json.dumps(user_json, default=str))

        return jsonify({"id": str(result.inserted_id)}), 201

    except Exception as e:
        return jsonify({"error": str(e)}), 500


@app.route("/user/<id>", methods=["GET"])
def get_user(id):
    try:
        user = redis_client.get(f"user:{id}")

        if user:
            return jsonify(user)

        user = users_collection.find_one({"id": id})

        if user:
            del user["_id"]

            redis_client.set(f"user:{user['id']}", json.dumps(user, default=str))

            return jsonify(user)
        else:
            return jsonify({"error": "User not found"}), 404

    except Exception as e:
        return jsonify({"error": str(e)}), 500


@app.route("/users", methods=["GET"])
def get_all_users():
    try:
        users_keys = redis_client.keys('*')

        if users_keys:
            users = [json.loads(redis_client.get(key)) for key in users_keys]
            return jsonify(users)

        users = list(users_collection.find())
        populate_cache(users)

        for user in users:
            del user["_id"]
        return jsonify(users)

    except Exception as e:
        return jsonify({"error": str(e)}), 500


@app.route("/user/<id>", methods=["PUT"])
def update_user(id):
    try:
        updated_data = request.json
        users_collection.update_one(
            {"id": id}, {"$set": updated_data})

        updated_data["id"] = id

        redis_client.set(f"user:{id}", json.dumps(updated_data, default=str))

        return jsonify({"message": "User updated successfully"})

    except Exception as e:
        return jsonify({"error": str(e)}), 500


@app.route("/user/<id>", methods=["DELETE"])
def delete_user(id):
    try:
        users_collection.delete_many({"id": id})
        redis_client.delete(f"user:{id}")

        return jsonify({"message": "User deleted successfully"})

    except Exception as e:
        return jsonify({"error": str(e)}), 500


if __name__ == "__main__":
    bjoern.run(app, "0.0.0.0", 8000, reuse_ports=True)
