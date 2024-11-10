-- 1. Create the database
CREATE DATABASE IF NOT EXISTS test_performance;
USE test_performance;

SELECT 'Database and schema created successfully' AS message;

-- 2. Create tables
CREATE TABLE IF NOT EXISTS large_table (
    id INT AUTO_INCREMENT PRIMARY KEY,
    data VARCHAR(255),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS related_table (
    id INT AUTO_INCREMENT PRIMARY KEY,
    large_table_id INT,
    description TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (large_table_id) REFERENCES large_table(id) ON DELETE CASCADE
);

SELECT 'Tables large_table and related_table created successfully' AS message;

-- 3. Drop procedures if they already exist
DROP PROCEDURE IF EXISTS InsertLargeData;
DROP PROCEDURE IF EXISTS InsertRelatedData;

SELECT 'Old procedures dropped (if existed)' AS message;

-- 4. Create and execute procedure to insert massive data into large_table
DELIMITER //
CREATE PROCEDURE InsertLargeData()
BEGIN
    DECLARE i INT DEFAULT 1;
    WHILE i <= 100000 DO
        INSERT INTO large_table (data) VALUES (CONCAT('Data ', FLOOR(1 + RAND() * 100000)));
        SET i = i + 1;
    END WHILE;
END;
//
DELIMITER ;

CALL InsertLargeData();
SELECT '100,000 rows inserted into large_table' AS message;

-- 5. Create and execute procedure to insert massive data into related_table
DELIMITER //
CREATE PROCEDURE InsertRelatedData()
BEGIN
    DECLARE i INT DEFAULT 1;
    DECLARE valid_id INT;
    
    WHILE i <= 50000 DO
        -- Select a random existing ID from `large_table`
        SELECT id INTO valid_id FROM large_table ORDER BY RAND() LIMIT 1;
        
        INSERT INTO related_table (large_table_id, description)
        VALUES (valid_id, CONCAT('Description for ID ', valid_id));
        
        SET i = i + 1;
    END WHILE;
END;
//
DELIMITER ;

CALL InsertRelatedData();
SELECT '50,000 rows inserted into related_table' AS message;

-- 6. Delete related rows in `related_table` before deleting from `large_table`
DELETE FROM related_table
WHERE large_table_id IN (SELECT id FROM large_table WHERE id % 7 = 0);
SELECT 'Related rows deleted from related_table' AS message;

-- 7. Perform massive deletions in `large_table` after ensuring no foreign key conflicts
DELETE FROM large_table
WHERE id % 7 = 0;
SELECT 'Massive deletion performed on large_table' AS message;

-- 8. Perform massive updates in `large_table`
UPDATE large_table
SET data = CONCAT(data, ' - Updated')
WHERE id % 5 = 0;
SELECT 'Massive update performed on large_table' AS message;

-- 9. Heavy query to induce a delay (30 seconds) using SLEEP
SELECT COUNT(*), SLEEP(30) AS delay
FROM large_table
WHERE data LIKE 'Data%';
SELECT 'Heavy query with 30-second delay completed' AS message;

-- 10. Another heavy query with a 45-second delay
SELECT large_table.id, COUNT(related_table.id), SLEEP(45) AS delay
FROM large_table
JOIN related_table ON large_table.id = related_table.large_table_id
GROUP BY large_table.id
HAVING COUNT(related_table.id) > 1;
SELECT 'Heavy query with 45-second delay completed' AS message;
