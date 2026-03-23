CREATE DATABASE IF NOT EXISTS user;

-- 
drop table IF EXISTS t_users_refresh_tokens;
CREATE TABLE t_users_refresh_tokens (
    id          INT AUTO_INCREMENT PRIMARY KEY,
    id_user     INT NOT NULL,
    token_hash  VARCHAR(255) NOT NULL,
    expires_at  DATETIME NOT NULL,
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- CONSTRAINT fk_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    -- INDEX idx_token_hash (token_hash)

drop table IF EXISTS t_roles;
CREATE TABLE t_roles (
    id INT PRIMARY KEY AUTO_INCREMENT,
    name VARCHAR(50) NOT NULL UNIQUE,
    slug VARCHAR(50) NOT NULL UNIQUE,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

insert into t_roles values(null, 'Admin', 'admin', default);
insert into t_roles values(null, 'User', 'user', default);

drop table IF EXISTS t_user;
CREATE TABLE t_users (
    id INT PRIMARY KEY AUTO_INCREMENT,
    name VARCHAR(250) NOT NULL,
    email VARCHAR(250) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    is_active BOOLEAN DEFAULT TRUE,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at DATETIME NULL DEFAULT NULL
);

drop table IF EXISTS t_user_roles;
CREATE TABLE t_user_roles (
    user_id INT NOT NULL,
    role_id INT NOT NULL,
    PRIMARY KEY (user_id, role_id)
);

insert into t_user_roles values(1, 1);
insert into t_user_roles values(2, 2);
-- SELECT id FROM t_roles WHERE slug IN ('admin', 'user');
-- CREATE TABLE t_user_roles (
--     user_id INT NOT NULL,
--     role_id INT NOT NULL,
--     PRIMARY KEY (user_id, role_id),
--     FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
--     FOREIGN KEY (role_id) REFERENCES roles(id) ON DELETE CASCADE
-- );

-- ALTER TABLE T_user ADD deleted_at DATETIME NULL DEFAULT NULL;
-- ALTER TABLE T_user DROP COLUMN deleted_at;
-- CREATE INDEX idx_users_active_created ON t_users (deleted_at, created_at DESC);

select * from t_roles;

insert into t_users values(null, 'Admin', 'admin@mail.com','pwd', default, default, default, null);
insert into t_users values(null, 'Jacky', 'Jacky@mail.com','pwd', default, default, default, null);

docker exec -it example-db-1 mysql -u root -p -e "USE user; select * from t_users;"
docker exec -it example-db-1 mysql -u root -p -e "USE user; SELECT id, name, email, created_at, updated_at 
        FROM t_users 
        WHERE deleted_at IS NULL
        ORDER BY created_at DESC
        LIMIT 5 OFFSET 3;"
docker exec -it example-db-1 mysql -u root -p -e "USE user; TRUNCATE TABLE t_users;"
docker exec -it example-db-1 mysql -u root -p -e "USE user; select * from t_users_refresh_tokens;"
