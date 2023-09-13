provider "azurerm" {
  features {}
}

resource "azurerm_resource_group" "crud" {
  name     = "crud"
  location = "westeurope"
}

resource "azurerm_virtual_network" "crud-vnet" {
  name                = "crud-vnet"
  resource_group_name = azurerm_resource_group.crud.name
  location            = azurerm_resource_group.crud.location
  address_space       = ["10.0.0.0/16"]
}

resource "azurerm_subnet" "crud-subnet" {
  name                 = "crud-subnet"
  resource_group_name  = azurerm_resource_group.crud.name
  virtual_network_name = azurerm_virtual_network.crud-vnet.name
  address_prefixes     = ["10.0.0.0/23"]
}


# Container storage 
resource "azurerm_storage_account" "crud-storage" {
  name                = "simplecrudapplication"
  resource_group_name = azurerm_resource_group.crud.name
  location            = azurerm_resource_group.crud.location

  account_tier             = "Standard"
  account_replication_type = "LRS"

}

resource "azurerm_storage_share" "crud-storage-share" {
  name                 = "mongodata"
  storage_account_name = azurerm_storage_account.crud-storage.name
  quota                = 20
}

resource "azurerm_container_app_environment" "crud-container-env" {
  name                           = "CRUD-Environment"
  location                       = azurerm_resource_group.crud.location
  resource_group_name            = azurerm_resource_group.crud.name
  infrastructure_subnet_id       = azurerm_subnet.crud-subnet.id
  internal_load_balancer_enabled = true
}

#### DNS

resource "azurerm_private_dns_zone" "crud-dns" {
  name                = azurerm_container_app_environment.crud-container-env.default_domain
  resource_group_name = azurerm_resource_group.crud.name
}

resource "azurerm_private_dns_zone_virtual_network_link" "crud-dns-vnet-link" {
  name                  = "vnet_link"
  resource_group_name   = azurerm_resource_group.crud.name
  private_dns_zone_name = azurerm_private_dns_zone.crud-dns.name
  virtual_network_id    = azurerm_virtual_network.crud-vnet.id
}

resource "azurerm_private_dns_a_record" "crud-ingress-record" {
  name                = "*"
  zone_name           = azurerm_private_dns_zone.crud-dns.name
  resource_group_name = azurerm_resource_group.crud.name
  ttl                 = 300
  records             = [azurerm_container_app_environment.crud-container-env.static_ip_address]
}

####


resource "azurerm_container_app_environment_storage" "crud-container-env-storage" {
  name                         = "container-storage"
  container_app_environment_id = azurerm_container_app_environment.crud-container-env.id
  account_name                 = azurerm_storage_account.crud-storage.name
  share_name                   = azurerm_storage_share.crud-storage-share.name
  access_key                   = azurerm_storage_account.crud-storage.primary_access_key
  access_mode                  = "ReadWrite"
}

resource "azurerm_container_app" "db" {
  name                         = "db"
  container_app_environment_id = azurerm_container_app_environment.crud-container-env.id
  resource_group_name          = azurerm_resource_group.crud.name
  revision_mode                = "Single"

  template {


    container {
      name   = "mongodb"
      image  = "mongo:latest"
      cpu    = "2"
      memory = "4Gi"

      volume_mounts {
        name = "db"
        path = "/data"

      }

      env {
        name  = "MONGO_INITDB_ROOT_USERNAME"
        value = "root"
      }
      env {
        name  = "MONGO_INITDB_ROOT_PASSWORD"
        value = "testpassword"
      }
    }

    min_replicas = 1

    volume {
      name         = "db"
      storage_type = "AzureFile"
      storage_name = azurerm_container_app_environment_storage.crud-container-env-storage.name

    }
  }
}

## TCP ingress
resource "terraform_data" "ingress_db" {
  triggers_replace = {
    timestamp = "${timestamp()}"
  }
  provisioner "local-exec" {
    command = "az containerapp ingress enable  --resource-group ${azurerm_resource_group.crud.name} --name ${azurerm_container_app.db.name}  --target-port 27017  --type external --exposed-port 27017 --transport tcp"
  }

  depends_on = [azurerm_container_app.db]
}


resource "terraform_data" "ingress_fqdn" {
  triggers_replace = {
    timestamp = terraform_data.ingress_db.id
  }
  provisioner "local-exec" {
    command = "az containerapp ingress show --resource-group ${azurerm_resource_group.crud.name} --name db --output json | jq -r  '.fqdn' > fqdn.txt"
  }
  depends_on = [terraform_data.ingress_db]
}

data "local_file" "fqdn" {
  filename   = "${path.module}/fqdn.txt"
  depends_on = [terraform_data.ingress_fqdn]
}

resource "azurerm_container_app" "app" {
  name                         = "app"
  container_app_environment_id = azurerm_container_app_environment.crud-container-env.id
  resource_group_name          = azurerm_resource_group.crud.name
  revision_mode                = "Single"

  template {

    container {
      name   = "app"
      image  = "lecz0/simple-crud-python"
      cpu    = "0.25"
      memory = "0.5Gi"
      env {
        name  = "MONGO_URI"
        value = "mongodb://root:testpassword@${chomp(data.local_file.fqdn.content)}:27017"
      }
    }

    min_replicas = 1
    max_replicas = 1

  }
  ingress {
    external_enabled           = true
    target_port                = 8080
    allow_insecure_connections = true
    traffic_weight {
      percentage      = 100
      latest_revision = true
    }
  }
  depends_on = [data.local_file.fqdn]
}


resource "azurerm_public_ip" "locust-vm-pub-ip" {
  name                = "public-ip"
  resource_group_name = azurerm_resource_group.crud.name
  location            = azurerm_resource_group.crud.location
  allocation_method   = "Static"
}

resource "azurerm_network_interface" "locust-vm-nic" {
  name                = "vm-nic"
  location            = azurerm_resource_group.crud.location
  resource_group_name = azurerm_resource_group.crud.name

  ip_configuration {
    name                          = "internal"
    subnet_id                     = azurerm_subnet.crud-subnet.id
    private_ip_address_allocation = "Dynamic"
    public_ip_address_id          = azurerm_public_ip.locust-vm-pub-ip.id
  }
}

resource "azurerm_linux_virtual_machine" "locust-vm" {
  name                = "locust-machine"
  resource_group_name = azurerm_resource_group.crud.name
  location            = azurerm_resource_group.crud.location
  size                = "Standard_B2ms"
  admin_username      = "adminuser"
  admin_password      = "Passw0rd123"
  network_interface_ids = [
    azurerm_network_interface.locust-vm-nic.id,
  ]

  admin_ssh_key {
    username   = "adminuser"
    public_key = file("~/.ssh/id_rsa.pub")
  }

  os_disk {
    caching              = "ReadWrite"
    storage_account_type = "Standard_LRS"
  }

  source_image_reference {
    publisher = "Canonical"
    offer     = "0001-com-ubuntu-server-focal"
    sku       = "20_04-lts"
    version   = "latest"
  }

  provisioner "file" {
    source      = "locustfile.py"
    destination = "/home/adminuser/locustfile.py"

    connection {
      type        = "ssh"
      user        = "adminuser"
      password    = "Passw0rd123"
      host        = azurerm_public_ip.locust-vm-pub-ip.ip_address
    }

  }
  provisioner "file" {
    source      = "docker-compose-locust.yml"
    destination = "/home/adminuser/docker-compose.yml"

    connection {
      type        = "ssh"
      user        = "adminuser"
      password    = "Passw0rd123"
      host        = azurerm_public_ip.locust-vm-pub-ip.ip_address
    }
  }
  depends_on = [azurerm_network_interface.locust-vm-nic]

}

resource "azurerm_virtual_machine_extension" "docker-env-provisioning" {
  name                 = "hostname"
  virtual_machine_id   = azurerm_linux_virtual_machine.locust-vm.id
  publisher            = "Microsoft.Azure.Extensions"
  type                 = "CustomScript"
  type_handler_version = "2.0"

  settings = <<SETTINGS
 {
  "commandToExecute": "curl -fsSL https://get.docker.com -o get-docker.sh && sudo sh get-docker.sh"
 }
SETTINGS
}

output "vm-ip" {
  value = azurerm_public_ip.locust-vm-pub-ip.ip_address 
}