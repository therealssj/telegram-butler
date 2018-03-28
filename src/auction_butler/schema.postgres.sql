-- Users do not get deleted from the database. Only `enlisted` switches to
-- false if the user leaves the group.
CREATE TABLE botuser (
  id         INT PRIMARY KEY NOT NULL, -- telegram user id
  username   TEXT,
  first_name TEXT,
  last_name  TEXT,
  enlisted   BOOL            NOT NULL DEFAULT TRUE, -- is in the group
  banned     BOOL            NOT NULL DEFAULT FALSE, -- is disabled even if in the group
  admin      BOOL            NOT NULL DEFAULT FALSE  -- can issue commands
);


create table auction (
  id INT AUTO_INCREMENT PRIMARY KEY NOT NULL, -- auto incrementing auction id
  end_time TIMESTAMP WITH TIME zone, -- auction end time
);