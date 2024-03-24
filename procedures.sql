DELIMITER //

DROP PROCEDURE IF EXISTS UPDATE_BITSKIN //
DROP PROCEDURE IF EXISTS debug_msg //
DROP PROCEDURE IF EXISTS GET_BITSKIN //

CREATE PROCEDURE UPDATE_BITSKIN (IN INP_ID BIGINT(20), IN INP_NAME VARCHAR(255),IN INP_LOWEST_PRICE BIGINT(20))
 BEGIN
    IF NOT EXISTS(SELECT 1 FROM BITSKINS WHERE ID=INP_ID) THEN
        INSERT INTO BITSKINS(ID,NAME,LOWEST_PRICE) VALUES (INP_ID,INP_NAME,INP_LOWEST_PRICE);
    ELSE
        UPDATE BITSKINS
        SET NAME=INP_NAME, LOWEST_PRICE=INP_LOWEST_PRICE
        WHERE ID=INP_ID;
    END IF;
 END;
//
CREATE PROCEDURE debug_msg(enabled INTEGER, msg VARCHAR(255))
BEGIN
  IF enabled THEN
    select concat('** ', msg) AS '** DEBUG:';
  END IF;
END
//
CREATE PROCEDURE GET_BITSKIN(IN SKIN_NAME VARCHAR(255),IN GUN_NAME VARCHAR(255),IN INP_TIER VARCHAR(191))
    BEGIN
        SELECT * 
        FROM BITSKINS
        WHERE LCASE(NAME) LIKE LCASE(CONCAT('%',SKIN_NAME,'%')) AND LCASE(NAME) LIKE LCASE(CONCAT('%',GUN_NAME,'%')) AND LCASE(NAME) LIKE LCASE(CONCAT('%',INP_TIER,'%'));
    END;
//
