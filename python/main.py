from flask import Flask, request, jsonify
from pymongo import MongoClient
from marshmallow_dataclass import dataclass
from dataclasses import  asdict
import bjoern
import os


app = Flask(__name__)

mongoURI = os.environ.get('MONGO_URI')
client = MongoClient(mongoURI)


db = client["crud_db"]
users_collection = db["users"]


@dataclass
class User:
    id:str
    name:str

@app.route("/user", methods=["POST"])
def create_user():
    try:
        user_data = request.json
        user = User.Schema().load(user_data)
        result = users_collection.insert_one(asdict(user))

        return jsonify({"id": str(result.inserted_id)}), 201
    except Exception as e:
        return jsonify({"error": str(e)}), 500


@app.route("/user/<id>", methods=["GET"])
def get_user(id):
    try:
        user = users_collection.find_one({"id": id})
        if user:
            del user["_id"]
            return jsonify(user)
        else:
            return jsonify({"error": "User not found"}), 404
    except Exception as e:
        return jsonify({"error": str(e)}), 500


@app.route("/users", methods=["GET"])
def get_all_users():
    try:
        users = list(users_collection.find())
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
        return jsonify({"message": "User updated successfully"})
    except Exception as e:
        return jsonify({"error": str(e)}), 500


@app.route("/user/<id>", methods=["DELETE"])
def delete_user(id):
    try:
        users_collection.delete_one({"id": id})
        return jsonify({"message": "User deleted successfully"})
    except Exception as e:
        return jsonify({"error": str(e)}), 500


if __name__ == "__main__":
    bjoern.run(app, "0.0.0.0", 8080,  reuse_port=True)

