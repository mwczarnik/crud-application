from locust import FastHttpUser, task, between
import random
import string

users = {}


def generate_name(length):
    return ''.join(random.choices(string.ascii_lowercase, k=length))


class UserBehavior(FastHttpUser):
    wait_time = between(1, 2.5)

    @task(6)
    def post_user(self):
        user_id = 0
        user_name = ''
        while True:
            user_id = random.randint(1, 100000000)
            user_name = generate_name(length=25)
            if user_id != 0 and user_id not in users:
                break

        users[user_id] = user_name
        self.client.post(
            "/user", json={"id": str(user_id), "name": str(user_name)})

    @task(1)
    def get_users(self):
        self.client.get("/users")

    @task(20)
    def get_user(self):
        users_ids = [*users]
        user_id = users_ids[random.randint(0, len(users_ids) - 1)]

        self.client.get(
            "/user/" + str(users_ids[user_id]))

    @task(16)
    def put_user(self):
        user_name = generate_name(length=25)
        users_ids = [*users]
        user_id = users_ids[random.randint(0, len(users_ids) - 1)]
        
        self.client.put(
            "/user/" + str(users_ids[user_id]), json={"name": user_name})

    @task(16)
    def delete_user(self):
        users_ids = [*users]
        if len(users_ids) > 0:
            user_id = users_ids[0]
            del users[user_id]
            self.client.delete("/user/" + str(user_id))
